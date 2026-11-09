// Simple reference server for RAZ-RCP-1 readers.
//
// It speaks the ADARAZ Cloud device API (/device/v1), so a reader can be
// pointed at it in its panel ("Wlasny serwer chmury"). Standard library
// only, data kept in one JSON file. Meant to be read and copied into an
// integrator's own system.
//
//	device.go   what the reader calls (/device/v1)
//	admin.go    what the integrator's system calls (/admin)
//	protocol.go codes, UIDs, time, users.bin, CRC, ETags
//	store.go    the data and its JSON file
package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

// #region routes
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	// The reader. Only unknown paths answer 404 - to a reader a 404 means
	// "wrong URL" and it keeps everything for later.
	mux.HandleFunc("POST /device/v1/claim", s.claim)
	mux.Handle("POST /device/v1/punch", s.device(s.punch))
	mux.Handle("GET /device/v1/users", s.device(s.users))
	mux.Handle("GET /device/v1/presence", s.device(s.presence))
	mux.Handle("POST /device/v1/seen", s.device(s.seen))
	mux.Handle("GET /device/v1/hello", s.device(s.hello))

	// The integrator's system.
	mux.Handle("POST /admin/pairing", s.admin(s.adminPairing))
	mux.Handle("PUT /admin/employees", s.admin(s.adminPutEmployees))
	mux.Handle("GET /admin/employees", s.admin(s.adminGetEmployees))
	mux.Handle("GET /admin/punches", s.admin(s.adminPunches))
	mux.Handle("GET /admin/devices", s.admin(s.adminDevices))
	mux.Handle("DELETE /admin/devices/{id}", s.admin(s.adminDeleteDevice))
	mux.Handle("GET /admin/seen", s.admin(s.adminSeen))
	return mux
}

// #endregion routes

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func main() {
	publicURL := os.Getenv("PUBLIC_URL") // exactly what is typed into the reader
	token := os.Getenv("ADMIN_TOKEN")
	if publicURL == "" || len(token) < 32 {
		log.Fatal("set PUBLIC_URL (e.g. https://rcp.example.com) and ADMIN_TOKEN (32+ characters)")
	}
	s, err := newServer(config{
		PublicURL:  publicURL,
		AdminToken: token,
		OrgName:    env("ORG_NAME", "Moja firma"),
		TZ:         env("TZ_NAME", "Europe/Warsaw"),
		DataFile:   env("DATA_FILE", "data.json"),
	})
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr:              env("LISTEN", ":8080"),
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second, // readers send a batch over one connection
	}
	log.Printf("listening on %s, readers use %s", srv.Addr, s.cfg.PublicURL)
	log.Fatal(srv.ListenAndServe())
}
