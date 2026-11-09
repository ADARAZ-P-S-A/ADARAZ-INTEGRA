package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// admin checks the integrator's token: Authorization: Bearer ADMIN_TOKEN.
func (s *server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.AdminToken)) != 1 {
			note(r, "bad admin token")
			writeError(w, http.StatusUnauthorized, "bad_admin_token", "bad admin token")
			return
		}
		next(w, r)
	}
}

// adminPairing: POST /admin/pairing {name?} -> {code, expires_at}. The code
// is typed into the reader's panel; it lives 15 minutes and works once.
func (s *server) adminPairing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // the body is optional
	s.mu.Lock()
	defer s.mu.Unlock()
	code := newCode()
	for s.db.Codes[code] != nil {
		code = newCode()
	}
	exp := s.now().Unix() + pairingTTL
	s.db.Codes[code] = &PairingCode{Name: strings.TrimSpace(req.Name), ExpiresAt: exp}
	if err := s.save(); err != nil {
		writeError(w, http.StatusInternalServerError, "unexpected", err.Error())
		return
	}
	note(r, "pairing code %s for %q, valid until %s", code, req.Name, fmtTime(exp))
	writeJSON(w, http.StatusCreated, map[string]any{"code": code, "expires_at": time.Unix(exp, 0).UTC()})
}

// adminPutEmployees: PUT /admin/employees [{id, name, cards}] replaces the
// whole list. The integrator's system is the truth; this is what every
// reader gets as users.bin on its next sync.
func (s *server) adminPutEmployees(w http.ResponseWriter, r *http.Request) {
	var list []Employee
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "expected [{id, name, cards}]")
		return
	}
	ids, cards := map[string]bool{}, map[string]bool{}
	for i := range list {
		e := &list[i]
		if e.ID == "" || ids[e.ID] {
			writeError(w, http.StatusBadRequest, "bad_employee", "every employee needs a unique id: "+e.ID)
			return
		}
		ids[e.ID] = true
		for j, c := range e.Cards {
			uid := normUID(c)
			if uid == "" || cards[uid] {
				writeError(w, http.StatusBadRequest, "bad_card", "bad or repeated card: "+c)
				return
			}
			cards[uid] = true
			e.Cards[j] = uid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Employees = list
	if err := s.save(); err != nil {
		writeError(w, http.StatusInternalServerError, "unexpected", err.Error())
		return
	}
	note(r, "employee list replaced: %d people, %d cards", len(list), len(cards))
	writeJSON(w, http.StatusOK, map[string]int{"employees": len(list), "cards": len(cards)})
}

func (s *server) adminGetEmployees(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, append([]Employee{}, s.db.Employees...))
}

// adminPunches: GET /admin/punches?after=<id> - every punch after that id,
// oldest first. Poll it with the last id you stored.
func (s *server) adminPunches(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []map[string]any{}
	for _, p := range s.db.Punches {
		if p.ID > after {
			out = append(out, map[string]any{
				"id":          p.ID,
				"device_id":   p.DeviceID,
				"uid":         p.UID,
				"employee_id": p.EmployeeID,
				"ts":          time.Unix(p.TS, 0).UTC(),
				"received":    time.Unix(p.Received, 0).UTC(),
			})
		}
	}
	note(r, "after=%d: %d punches", after, len(out))
	writeJSON(w, http.StatusOK, out)
}

func (s *server) adminDevices(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []map[string]any{}
	for _, d := range s.db.Devices {
		out = append(out, map[string]any{
			"id": d.ID, "name": d.Name, "login": d.Login,
			"serial": d.Serial, "firmware": d.Firmware, "last_seen": d.LastSeen,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// adminDeleteDevice: DELETE /admin/devices/{id}. Its credentials stop
// working at once (401); its punches stay.
func (s *server) adminDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, d := range s.db.Devices {
		if d.ID == id {
			s.db.Devices = append(s.db.Devices[:i], s.db.Devices[i+1:]...)
			if err := s.save(); err != nil {
				writeError(w, http.StatusInternalServerError, "unexpected", err.Error())
				return
			}
			note(r, "device %q deleted, its credentials stop working", d.Name)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "no such device")
}

// adminSeen: GET /admin/seen - cards tapped on a reader that nobody holds.
func (s *server) adminSeen(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []map[string]any{}
	for uid, c := range s.db.Seen {
		if s.holder(uid) == "" {
			out = append(out, map[string]any{
				"uid": uid, "device_id": c.DeviceID, "count": c.Count, "denied": c.Denied,
				"last_seen": time.Unix(c.LastSeen, 0).UTC(),
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}
