// Minimal RAZ-RCP-1 punch receiver (Go 1.21+, standard library only).
// Reader: Settings > Integrations > Synchronization, format http3,
// URL http://<this computer>:8080/odbicia, login rcp, password = RCP_HASLO.
package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	login, haslo = "rcp", os.Getenv("RCP_HASLO")
	rfidRe       = regexp.MustCompile(`^[0-9a-f]{1,20}$`)
	mu           sync.Mutex
	seen         = map[string]bool{}
)

func answer(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprint(w, body)
}

func punch(w http.ResponseWriter, r *http.Request) {
	user, pass, _ := r.BasicAuth()
	if subtle.ConstantTimeCompare([]byte(user+":"+pass), []byte(login+":"+haslo)) != 1 {
		answer(w, 401, `{"error":"unauthorized"}`) // punch stays in the reader
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	rfid, machine := r.FormValue("rfidID"), r.FormValue("machineId")
	ts, err := time.Parse("2006-01-02 15:04:05", r.FormValue("timestamp")) // http3: UTC
	if !rfidRe.MatchString(rfid) || err != nil {
		answer(w, 400, `{"error":"bad request"}`) // dropped by the reader
		return
	}
	if ts.Year() < 2026 {
		answer(w, 200, `{"stored":false,"reason":"clock"}`)
		return
	}
	key := fmt.Sprintf("%d\t%s\t%s", ts.Unix(), rfid, strings.NewReplacer("\r", " ", "\n", " ").Replace(machine))
	mu.Lock()
	defer mu.Unlock()
	if seen[key] {
		answer(w, 200, `{"stored":false,"reason":"duplicate"}`)
		return
	}
	f, err := os.OpenFile("odbicia.tsv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // store first...
	if err == nil {
		_, err = f.WriteString(key + "\n")
		if err == nil {
			err = f.Sync()
		}
		f.Close()
	}
	if err != nil {
		answer(w, 500, `{"error":"storage"}`) // the reader retries later
		return
	}
	seen[key] = true
	answer(w, 200, `{"stored":true}`) // ...then answer 200
}

func main() {
	http.HandleFunc("POST /odbicia", punch)
	http.ListenAndServe(":8080", nil)
}
