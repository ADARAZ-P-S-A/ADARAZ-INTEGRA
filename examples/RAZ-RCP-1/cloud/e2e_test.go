package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testToken = "test-admin-token-0123456789abcdef0123"

// TestE2E runs the real HTTP server and plays both sides: the integrator's
// system (/admin) and a RAZ-RCP-1 reader (/device/v1), doing what the
// firmware does - pair with a code, take the list of people, send punches,
// ask who is inside, report an unknown card.
func TestE2E(t *testing.T) {
	s, err := newServer(config{AdminToken: testToken, OrgName: "Firma Test", TZ: "Europe/Warsaw",
		DataFile: filepath.Join(t.TempDir(), "data.json")})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(logRequests(s.routes()))
	defer ts.Close()
	s.cfg.PublicURL = ts.URL

	// 1. The integrator makes a pairing code; the installer types it into
	// the reader (any case, with a space instead of the dash).
	var code struct{ Code string }
	call(t, "POST", ts.URL+"/admin/pairing", adminAuth, "", 201, &code)

	// 2. The reader claims it, with the body the firmware sends.
	claimBody := `{"code":"` + strings.ToLower(strings.Replace(code.Code, "-", " ", 1)) + `","serial":"ef4924d4","firmware":"121"}`
	var claim struct {
		Organization string
		API          string
		Credentials  credentials
	}
	call(t, "POST", ts.URL+"/device/v1/claim", nil, claimBody, 200, &claim)
	if claim.Organization != "Firma Test" || claim.API != ts.URL+"/device/v1" ||
		claim.Credentials.URL != ts.URL+"/device/v1/punch" || claim.Credentials.Format != 3 {
		t.Fatalf("claim answer: %+v", claim)
	}
	reader := basic(claim.Credentials.Login, claim.Credentials.Password)
	api := claim.API

	// 3. The integrator sends the people and their cards.
	call(t, "PUT", ts.URL+"/admin/employees", adminAuth,
		`[{"id":"E1","name":"Anna","cards":["04A1B2C3"]},{"id":"E2","name":"Jan","cards":["04:b1:b2:b3:b4:b5:b6"]}]`, 200, nil)

	// 4. The reader takes users.bin and checks it like the firmware does.
	resp, body := do(t, "GET", api+"/users", reader, "", nil)
	if resp.StatusCode != 200 || len(body) != 2*recordSize || resp.Header.Get("X-RCP-Users") != "2" {
		t.Fatalf("users: %d, %d bytes, X-RCP-Users %q", resp.StatusCode, len(body), resp.Header.Get("X-RCP-Users"))
	}
	for i := 0; i < len(body); i += recordSize {
		rec := body[i : i+recordSize]
		if crc := crc16(rec[:46]); rec[46] != byte(crc) || rec[47] != byte(crc>>8) || rec[1] < 4 || rec[1] > 7 {
			t.Fatalf("record %d broken: % x", i/recordSize, rec)
		}
	}
	hash := hash16(body) // the reader's If-None-Match is the hash of its own file
	if resp.Header.Get("ETag") != `"`+hash+`"` {
		t.Fatalf("ETag %s, want the hash of the body %s", resp.Header.Get("ETag"), hash)
	}
	if resp, _ := do(t, "GET", api+"/users", reader, "", map[string]string{"If-None-Match": `"` + hash + `"`}); resp.StatusCode != 304 {
		t.Fatalf("users with the reader's own hash: %d, want 304", resp.StatusCode)
	}

	// 5. Punches. Every well-formed one is a 200, whatever happened to it.
	now := time.Now().UTC()
	tap := now.Add(-time.Minute).Format("2006-01-02 15:04:05")
	punch := func(uid, when string) (int, string) {
		resp, b := do(t, "POST", api+"/punch", reader,
			url.Values{"rfidID": {uid}, "timestamp": {when}, "machineId": {"RAZ-RCP-1"}}.Encode(),
			map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
		return resp.StatusCode, string(b)
	}
	expect := func(what string, gotStatus int, gotBody string, status int, part string) {
		t.Helper()
		if gotStatus != status || !strings.Contains(gotBody, part) {
			t.Fatalf("%s: %d %s, want %d with %s", what, gotStatus, gotBody, status, part)
		}
	}
	st, b := punch("04a1b2c3", tap)
	expect("punch of Anna's card", st, b, 200, `"stored":true`)
	st, b = punch("04a1b2c3", tap)
	expect("the same punch again", st, b, 200, `"duplicate":true`)
	expect("compact JSON (the firmware looks for this substring)", st, b, 200, `"stored":false`)
	st, b = punch("04ffeeddcc", tap)
	expect("unknown card", st, b, 200, `"stored":true`)
	st, b = punch("zz11yy22", tap)
	expect("not hex", st, b, 400, "bad_uid")
	st, b = punch("04a1b2c3", "yesterday")
	expect("unreadable time", st, b, 400, "bad_time")
	st, b = punch("04a1b2c3", "2019-12-31 23:59:59")
	expect("clock not set", st, b, 200, `"reason":"clock"`)

	// 6. Who is inside: Anna punched once, so her bit is set and Jan's is not.
	resp, bits := do(t, "GET", api+"/presence", reader, "", nil)
	if resp.StatusCode != 200 || len(bits) != 1 || resp.Header.Get("X-RCP-Users-ETag") != `"`+hash+`"` {
		t.Fatalf("presence: %d % x, X-RCP-Users-ETag %s", resp.StatusCode, bits, resp.Header.Get("X-RCP-Users-ETag"))
	}
	inside := map[uint64]bool{}
	for i := 0; i < len(body)/recordSize; i++ {
		no := strings.TrimRight(string(body[i*recordSize+9:i*recordSize+46]), "\x00")
		inside[parseU(t, no)] = bits[i/8]&(1<<(i%8)) != 0
	}
	if !inside[empNo("E1")] || inside[empNo("E2")] {
		t.Fatalf("presence bits % x: Anna should be inside, Jan not", bits)
	}

	// 7. An unknown card the reader turned away.
	resp, _ = do(t, "POST", api+"/seen", reader, "rfidID=04c1c2c3&timestamp="+url.QueryEscape(tap)+"&denied=1",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if resp.StatusCode != 200 {
		t.Fatalf("seen: %d", resp.StatusCode)
	}
	var seen []map[string]any
	call(t, "GET", ts.URL+"/admin/seen", adminAuth, "", 200, &seen)
	if len(seen) != 1 || seen[0]["uid"] != "04c1c2c3" || seen[0]["denied"] != true {
		t.Fatalf("admin seen: %v", seen)
	}

	// 8. The integrator collects the punches.
	var punches []map[string]any
	call(t, "GET", ts.URL+"/admin/punches?after=0", adminAuth, "", 200, &punches)
	if len(punches) != 2 || punches[0]["employee_id"] != "E1" || punches[1]["employee_id"] != "" {
		t.Fatalf("admin punches: %v", punches)
	}

	// 9. Wrong password: 401 with WWW-Authenticate (the reader shows "not connected").
	resp, _ = do(t, "GET", api+"/users", basic(claim.Credentials.Login, "wrong"), "", nil)
	if resp.StatusCode != 401 || !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Basic") {
		t.Fatalf("bad password: %d %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	// 10. The code is spent: claiming it again after the reader used its
	// credentials is refused.
	call(t, "POST", ts.URL+"/device/v1/claim", nil, claimBody, 404, nil)

	// 11. Deleting the reader cuts it off.
	var devices []map[string]any
	call(t, "GET", ts.URL+"/admin/devices", adminAuth, "", 200, &devices)
	call(t, "DELETE", ts.URL+"/admin/devices/"+devices[0]["id"].(string), adminAuth, "", 204, nil)
	if resp, _ := do(t, "GET", api+"/hello", reader, "", nil); resp.StatusCode != 401 {
		t.Fatalf("deleted reader: %d, want 401", resp.StatusCode)
	}
}

// TestRetriedClaim: the answer to a claim got lost, the reader asks again
// with the same code - it gets a new password and the first one is dead.
func TestRetriedClaim(t *testing.T) {
	s, err := newServer(config{PublicURL: "http://192.168.1.10:8080", AdminToken: testToken,
		DataFile: filepath.Join(t.TempDir(), "data.json")})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(logRequests(s.routes()))
	defer ts.Close()

	var code struct{ Code string }
	call(t, "POST", ts.URL+"/admin/pairing", adminAuth, "", 201, &code)
	body := `{"code":"` + code.Code + `","serial":"A1","firmware":"121"}`
	var first, second struct{ Credentials credentials }
	call(t, "POST", ts.URL+"/device/v1/claim", nil, body, 200, &first)
	call(t, "POST", ts.URL+"/device/v1/claim", nil, `{"code":"`+code.Code+`","serial":"OTHER"}`, 404, nil)
	call(t, "POST", ts.URL+"/device/v1/claim", nil, body, 200, &second)
	if second.Credentials.Login != first.Credentials.Login || second.Credentials.Password == first.Credentials.Password {
		t.Fatal("a retried claim must keep the login and give a new password")
	}
	if resp, _ := do(t, "GET", ts.URL+"/device/v1/hello", basic(first.Credentials.Login, first.Credentials.Password), "", nil); resp.StatusCode != 401 {
		t.Fatalf("first password after the retry: %d, want 401", resp.StatusCode)
	}
	if resp, _ := do(t, "GET", ts.URL+"/device/v1/hello", basic(second.Credentials.Login, second.Credentials.Password), "", nil); resp.StatusCode != 200 {
		t.Fatalf("second password: %d, want 200", resp.StatusCode)
	}
}

func TestCRC16(t *testing.T) {
	if got := crc16([]byte("123456789")); got != 0x29B1 {
		t.Fatalf("crc16 = %#04x, want 0x29b1", got)
	}
}

// ---------------------------------------------------------------- helpers

type auth func(*http.Request)

func adminAuth(r *http.Request) { r.Header.Set("Authorization", "Bearer "+testToken) }

func basic(login, password string) auth {
	return func(r *http.Request) { r.SetBasicAuth(login, password) }
}

func do(t *testing.T, method, u string, a auth, body string, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, u, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if a != nil {
		a(req)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func call(t *testing.T, method, u string, a auth, body string, status int, out any) {
	t.Helper()
	resp, b := do(t, method, u, a, body, map[string]string{"Content-Type": "application/json"})
	if resp.StatusCode != status {
		t.Fatalf("%s %s: %d %s, want %d", method, u, resp.StatusCode, b, status)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: %v in %s", method, u, err, b)
		}
	}
}

func parseU(t *testing.T, v string) uint64 {
	t.Helper()
	var n uint64
	for _, c := range v {
		if c < '0' || c > '9' {
			t.Fatalf("emp_no %q is not decimal", v)
		}
		n = n*10 + uint64(c-'0')
	}
	return n
}
