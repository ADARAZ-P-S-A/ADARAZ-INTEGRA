package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The pairing code alphabet: no 0/O, no 1/I, it is read off a screen.
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

const (
	pairingTTL = 15 * 60 // seconds a pairing code lives, and a claim may be retried
	recordSize = 48      // one users.bin record
	readerMax  = 2048    // a reader takes at most this many records
)

var (
	uidRe         = regexp.MustCompile(`^([0-9a-f]{2}){4,7}$`)
	earliestPunch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
)

func newCode() string {
	b := randomBytes(8)
	out := make([]byte, 0, 9)
	for i, v := range b {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, codeAlphabet[int(v)%len(codeAlphabet)]) // 256 % 32 == 0: unbiased
	}
	return string(out)
}

// normCode accepts what a person types: any case, with or without the dash.
func normCode(v string) string {
	v = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(v)))
	if len(v) != 8 {
		return ""
	}
	for _, r := range v {
		if !strings.ContainsRune(codeAlphabet, r) {
			return ""
		}
	}
	return v[:4] + "-" + v[4:]
}

// normUID: 4..7 bytes of hex; case, ":", " " and "-" do not matter.
func normUID(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.NewReplacer(":", "", " ", "", "-", "").Replace(v)
	if !uidRe.MatchString(v) {
		return ""
	}
	return v
}

// parseTime reads what the reader sends: "YYYY-MM-DD HH:MM:SS" in UTC, or
// unix seconds, or RFC 3339.
func parseTime(v string) (int64, bool) {
	v = strings.TrimSpace(v)
	if n, err := strconv.ParseInt(v, 10, 64); err == nil && len(v) >= 9 {
		return n, true
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, v, time.UTC); err == nil {
			return t.Unix(), true
		}
	}
	return 0, false
}

// clockWrong: before 2020 or more than a day ahead - the reader's clock was
// not set. Such a punch is answered 200 and not stored: there is no honest
// time to file it under, and refusing it would block the reader's queue.
func clockWrong(ts, now int64) bool {
	return ts < earliestPunch || ts > now+24*3600
}

// crc16 is CRC-16/CCITT-FALSE (poly 0x1021, init 0xFFFF), as in the
// firmware's records.c. "123456789" gives 0x29B1.
func crc16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// empNo is the number the reader pairs in and out by. It only has to be
// stable per person and different between people: here the first 8 bytes
// of SHA-256 of the integrator's employee id.
func empNo(id string) uint64 {
	sum := sha256.Sum256([]byte(id))
	return binary.BigEndian.Uint64(sum[:8])
}

type listEntry struct {
	no       uint64
	uid      string
	employee string
}

// readerList is every card of users.bin in the reader's order: by emp_no
// as a number, then by UID. Call with mu held.
func (s *server) readerList() []listEntry {
	var list []listEntry
	for _, e := range s.db.Employees {
		for _, c := range e.Cards {
			list = append(list, listEntry{no: empNo(e.ID), uid: c, employee: e.ID})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].no != list[j].no {
			return list[i].no < list[j].no
		}
		return list[i].uid < list[j].uid
	})
	return list
}

// renderUsers builds users.bin: 48-byte records.
//
//	[0] 0x00   [1] UID length   [2..8] UID, zero-padded
//	[9..45]  emp_no in decimal ASCII, zero bytes after it
//	[46..47] CRC-16 over [0..45], little-endian
func renderUsers(list []listEntry) []byte {
	out := make([]byte, 0, len(list)*recordSize)
	for _, u := range list {
		raw, _ := hex.DecodeString(u.uid)
		var rec [recordSize]byte
		rec[1] = byte(len(raw))
		copy(rec[2:9], raw)
		copy(rec[9:46], strconv.FormatUint(u.no, 10))
		crc := crc16(rec[:46])
		rec[46] = byte(crc)
		rec[47] = byte(crc >> 8)
		out = append(out, rec[:]...)
	}
	return out
}

// hash16 is the first 16 bytes of SHA-256 as hex: the users.bin ETag (the
// reader sends the same hash of the file it has) and the presence ETag.
func hash16(parts ...[]byte) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// etagMatches: If-None-Match may hold several values, quoted or not, W/ or not.
func etagMatches(header, etag string) bool {
	for _, v := range strings.Split(header, ",") {
		v = strings.Trim(strings.TrimPrefix(strings.TrimSpace(v), "W/"), `"`)
		if v != "" && strings.EqualFold(v, etag) {
			return true
		}
	}
	return false
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func sha256hex(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

// writeJSON writes compact JSON. The reader looks for the exact substring
// "stored":false, so no spaces - encoding/json never adds any.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
