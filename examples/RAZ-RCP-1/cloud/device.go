package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// How the reader reads the status of every answer:
//
//	200       done (a punch: removed from the reader)
//	304       nothing changed (/users, /presence)
//	400       this one record can never be read - dropped, the rest goes on
//	401       bad credentials - "not connected", everything stays on the reader
//	404       wrong URL - everything stays on the reader
//	429, 5xx  try again on the next sync
//
// So a well-formed punch is ALWAYS a 200: an unknown card, a resend and an
// unset clock included.

// device checks HTTP Basic credentials (from the claim).
func (s *server) device(next func(http.ResponseWriter, *http.Request, *Device)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		login, password, ok := r.BasicAuth()
		s.mu.Lock()
		d := s.deviceByLogin(login)
		good := ok && d != nil &&
			subtle.ConstantTimeCompare([]byte(sha256hex(password)), []byte(d.SecretHash)) == 1
		if good {
			d.LastSeen = s.now().Unix()
		}
		s.mu.Unlock()
		if !good {
			note(r, "bad credentials login=%q", login)
			w.Header().Set("WWW-Authenticate", `Basic realm="rcp"`)
			writeError(w, http.StatusUnauthorized, "bad_device_credentials", "bad device credentials")
			return
		}
		note(r, "device=%q", d.Name)
		next(w, r, d)
	}
}

type credentials struct {
	URL         string `json:"url"`
	Login       string `json:"login"`
	Password    string `json:"password"`
	Format      int    `json:"format"`       // 3 = http3: the UID as hex
	IntervalMin int    `json:"interval_min"` // a suggestion; the reader decides
}

// claim: POST /device/v1/claim {code, serial, firmware}.
//
// No credentials: the code is the credential, and it is single use. The
// one exception is the same reader asking again because the answer never
// reached it: same code, same serial, within 15 minutes, before it ever
// used its credentials - it gets a NEW password and the old one is dead.
func (s *server) claim(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var req struct {
		Code     string `json:"code"`
		Serial   string `json:"serial"`
		Firmware string `json:"firmware"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "bad request")
		return
	}
	badCode := func() {
		note(r, "wrong, used or expired code")
		writeError(w, http.StatusNotFound, "bad_pairing_code", "this code is wrong, used or expired")
	}
	code := normCode(req.Code)
	note(r, "code=%q serial=%q firmware=%q", req.Code, req.Serial, req.Firmware)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().Unix()
	c := s.db.Codes[code]
	if code == "" || c == nil {
		badCode()
		return
	}
	password := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	var d *Device
	if c.ClaimedAt != 0 {
		d = s.deviceByID(c.DeviceID)
		if d == nil || req.Serial == "" || req.Serial != c.Serial ||
			now > c.ClaimedAt+pairingTTL || d.LastSeen != 0 {
			badCode()
			return
		}
		d.SecretHash = sha256hex(password)
		note(r, "retried claim: new password for device=%q", d.Name)
	} else {
		if now > c.ExpiresAt {
			badCode()
			return
		}
		name := c.Name
		if name == "" {
			name = strings.TrimSpace(req.Name)
		}
		if name == "" {
			name = strings.TrimSpace("RAZ-RCP-1 " + req.Serial)
		}
		d = &Device{
			ID:         hex.EncodeToString(randomBytes(16)),
			Name:       name,
			Login:      "rcp-" + hex.EncodeToString(randomBytes(6)),
			SecretHash: sha256hex(password),
			Serial:     req.Serial,
			Firmware:   req.Firmware,
		}
		s.db.Devices = append(s.db.Devices, d)
		c.ClaimedAt, c.DeviceID, c.Serial = now, d.ID, req.Serial
		note(r, "paired device=%q id=%s login=%s", d.Name, d.ID, d.Login)
	}
	if err := s.save(); err != nil {
		writeError(w, http.StatusInternalServerError, "unexpected", "something went wrong - try again")
		return
	}
	api := s.cfg.PublicURL + "/device/v1"
	writeJSON(w, http.StatusOK, map[string]any{
		"device":       map[string]string{"id": d.ID, "name": d.Name},
		"organization": s.cfg.OrgName,
		"credentials":  credentials{URL: api + "/punch", Login: d.Login, Password: password, Format: 3, IntervalMin: 1},
		"api":          api,
	})
}

// punch: POST /device/v1/punch, form rfidID + timestamp (UTC) + machineId
// (ignored: which reader it is comes from the login).
func (s *server) punch(w http.ResponseWriter, r *http.Request, d *Device) {
	uid := normUID(r.FormValue("rfidID"))
	if uid == "" {
		note(r, "bad rfidID=%q, dropped by the reader", r.FormValue("rfidID"))
		writeError(w, http.StatusBadRequest, "bad_uid", "rfidID is 4 to 7 bytes of hex")
		return
	}
	ts, ok := parseTime(r.FormValue("timestamp"))
	if !ok {
		note(r, "uid=%s bad timestamp=%q, dropped by the reader", uid, r.FormValue("timestamp"))
		writeError(w, http.StatusBadRequest, "bad_time", "timestamp is YYYY-MM-DD HH:MM:SS in UTC")
		return
	}
	note(r, "uid=%s ts=%s", uid, fmtTime(ts))
	now := s.now().Unix()
	if clockWrong(ts, now) {
		note(r, "not stored: the reader's clock was not set at the tap")
		writeJSON(w, http.StatusOK, map[string]any{"stored": false, "reason": "clock"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := dupKey(d.ID, uid, ts)
	if s.dups[key] {
		note(r, "duplicate (already stored)")
		writeJSON(w, http.StatusOK, map[string]any{"stored": false, "duplicate": true})
		return
	}
	s.db.Punches = append(s.db.Punches, Punch{
		ID:         int64(len(s.db.Punches)) + 1,
		DeviceID:   d.ID,
		UID:        uid,
		EmployeeID: s.holder(uid), // whoever holds the card now, i.e. at the punch
		TS:         ts,
		Received:   now,
	})
	if err := s.save(); err != nil {
		s.db.Punches = s.db.Punches[:len(s.db.Punches)-1]
		writeError(w, http.StatusInternalServerError, "unexpected", "something went wrong - try again")
		return
	}
	s.dups[key] = true
	if e := s.db.Punches[len(s.db.Punches)-1].EmployeeID; e != "" {
		note(r, "stored employee=%s", e)
	} else {
		note(r, "stored, unknown card")
	}
	writeJSON(w, http.StatusOK, map[string]any{"stored": true, "duplicate": false})
}

// users: GET /device/v1/users. The reader sends the hash of the users.bin
// it has in If-None-Match; equal means 304, so nothing is downloaded on
// most syncs.
func (s *server) users(w http.ResponseWriter, r *http.Request, d *Device) {
	s.mu.Lock()
	list := s.readerList()
	s.mu.Unlock()
	if len(list) > readerMax {
		log.Printf("users.bin has %d cards, a reader takes at most %d and keeps its old list", len(list), readerMax)
	}
	body := renderUsers(list)
	etag := hash16(body)
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("X-RCP-Users", strconv.Itoa(len(list)))
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		note(r, "employee list unchanged (%d cards)", len(list))
		w.WriteHeader(http.StatusNotModified)
		return
	}
	note(r, "sent employee list: %d cards", len(list))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(body)
}

// presence: GET /device/v1/presence - who is inside, one bit per users.bin
// record (record i = byte i/8, bit i%8). Inside = an odd number of punches,
// from any reader. X-RCP-Users-ETag tells the reader which list the bits
// are about; it applies them only to that list.
func (s *server) presence(w http.ResponseWriter, r *http.Request, d *Device) {
	s.mu.Lock()
	list := s.readerList()
	count := map[string]int{}
	for _, p := range s.db.Punches {
		if p.EmployeeID != "" {
			count[p.EmployeeID]++
		}
	}
	s.mu.Unlock()
	bitmap := make([]byte, (len(list)+7)/8)
	for i, u := range list {
		if count[u.employee]%2 == 1 {
			bitmap[i/8] |= 1 << (i % 8)
		}
	}
	usersTag := hash16(renderUsers(list))
	etag := hash16([]byte(usersTag), bitmap)
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("X-RCP-Users-ETag", `"`+usersTag+`"`)
	inside, seenEmp := 0, map[string]bool{} // people, not cards
	for _, u := range list {
		if count[u.employee]%2 == 1 && !seenEmp[u.employee] {
			seenEmp[u.employee] = true
			inside++
		}
	}
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		note(r, "presence unchanged (%d inside)", inside)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	note(r, "sent presence: %d inside", inside)
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(bitmap)
}

// seen: POST /device/v1/seen, form rfidID + timestamp + denied. A card that
// is not in the reader's users.bin was tapped - kept so it can be assigned.
func (s *server) seen(w http.ResponseWriter, r *http.Request, d *Device) {
	uid := normUID(r.FormValue("rfidID"))
	if uid == "" {
		note(r, "bad rfidID=%q", r.FormValue("rfidID"))
		writeError(w, http.StatusBadRequest, "bad_uid", "rfidID is 4 to 7 bytes of hex")
		return
	}
	ts, ok := parseTime(r.FormValue("timestamp"))
	if !ok {
		note(r, "uid=%s bad timestamp=%q", uid, r.FormValue("timestamp"))
		writeError(w, http.StatusBadRequest, "bad_time", "timestamp is YYYY-MM-DD HH:MM:SS in UTC")
		return
	}
	note(r, "unknown card uid=%s ts=%s denied=%s", uid, fmtTime(ts), r.FormValue("denied"))
	if clockWrong(ts, s.now().Unix()) {
		note(r, "not stored: the reader's clock was not set")
		writeJSON(w, http.StatusOK, map[string]any{"stored": false, "reason": "clock"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.db.Seen[uid]
	if c == nil {
		c = &SeenCard{}
		s.db.Seen[uid] = c
	}
	c.DeviceID = d.ID
	c.Denied = r.FormValue("denied") == "1" || r.FormValue("denied") == "true"
	c.Count++
	if ts > c.LastSeen {
		c.LastSeen = ts
	}
	if err := s.save(); err != nil {
		writeError(w, http.StatusInternalServerError, "unexpected", "something went wrong - try again")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stored": true})
}

// hello: GET /device/v1/hello - a connection test.
func (s *server) hello(w http.ResponseWriter, r *http.Request, d *Device) {
	writeJSON(w, http.StatusOK, map[string]any{
		"device":       d.Name,
		"organization": s.cfg.OrgName,
		"tz":           s.cfg.TZ,
		"epoch":        s.now().Unix(),
	})
}
