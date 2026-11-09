package main

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Employee struct {
	ID    string   `json:"id"`    // the integrator's own id (any string)
	Name  string   `json:"name"`  // never sent to the reader
	Cards []string `json:"cards"` // UIDs, lowercase hex
}

type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Login      string `json:"login"`
	SecretHash string `json:"secret_hash"` // hex SHA-256 of the password
	Serial     string `json:"serial"`
	Firmware   string `json:"firmware"`
	LastSeen   int64  `json:"last_seen"` // unix, 0 = never authenticated
}

type PairingCode struct {
	Name      string `json:"name"`
	ExpiresAt int64  `json:"expires_at"`
	ClaimedAt int64  `json:"claimed_at"`
	DeviceID  string `json:"device_id"`
	Serial    string `json:"serial"`
}

type Punch struct {
	ID         int64  `json:"id"`
	DeviceID   string `json:"device_id"`
	UID        string `json:"uid"`
	EmployeeID string `json:"employee_id"` // "" = a card nobody held at that moment
	TS         int64  `json:"ts"`
	Received   int64  `json:"received"`
}

type SeenCard struct {
	DeviceID string `json:"device_id"`
	LastSeen int64  `json:"last_seen"`
	Count    int    `json:"count"`
	Denied   bool   `json:"denied"`
}

// data is everything the server keeps, saved as one JSON file.
type data struct {
	Employees []Employee              `json:"employees"`
	Devices   []*Device               `json:"devices"`
	Codes     map[string]*PairingCode `json:"codes"`
	Punches   []Punch                 `json:"punches"`
	Seen      map[string]*SeenCard    `json:"seen"`
}

type config struct {
	PublicURL  string
	AdminToken string
	OrgName    string
	TZ         string
	DataFile   string
}

type server struct {
	cfg config
	now func() time.Time

	mu   sync.Mutex // guards db and dups
	db   data
	dups map[string]bool // device|uid|ts of every stored punch
}

func newServer(cfg config) (*server, error) {
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	s := &server{cfg: cfg, now: time.Now, dups: map[string]bool{}}
	raw, err := os.ReadFile(cfg.DataFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(raw, &s.db); err != nil {
			return nil, err
		}
	}
	if s.db.Codes == nil {
		s.db.Codes = map[string]*PairingCode{}
	}
	if s.db.Seen == nil {
		s.db.Seen = map[string]*SeenCard{}
	}
	for _, p := range s.db.Punches {
		s.dups[dupKey(p.DeviceID, p.UID, p.TS)] = true
	}
	return s, nil
}

// save writes the whole file atomically (temp file + rename). Call with mu held.
func (s *server) save() error {
	raw, err := json.MarshalIndent(&s.db, "", " ")
	if err != nil {
		return err
	}
	tmp := s.cfg.DataFile + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.cfg.DataFile)
}

func dupKey(device, uid string, ts int64) string {
	return device + "|" + uid + "|" + strconv.FormatInt(ts, 10)
}

// holder is who holds the card now. Call with mu held.
func (s *server) holder(uid string) string {
	for _, e := range s.db.Employees {
		for _, c := range e.Cards {
			if c == uid {
				return e.ID
			}
		}
	}
	return ""
}

func (s *server) deviceByLogin(login string) *Device {
	for _, d := range s.db.Devices {
		if d.Login == login {
			return d
		}
	}
	return nil
}

func (s *server) deviceByID(id string) *Device {
	for _, d := range s.db.Devices {
		if d.ID == id {
			return d
		}
	}
	return nil
}
