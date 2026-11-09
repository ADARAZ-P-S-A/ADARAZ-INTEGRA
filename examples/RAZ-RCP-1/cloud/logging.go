package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// One log line per request, from the reader and from your system:
//
//	POST /device/v1/punch 200 3ms 192.168.1.50 device="Wejscie" uid=04a1b2c3 ts=2026-11-09T06:58:12Z stored employee=E001
//
// Handlers add what happened with note(); passwords and tokens never go in.

type logNotes struct{ parts []string }

type notesKey struct{}

// note adds a detail to the log line of this request.
func note(r *http.Request, format string, a ...any) {
	if n, ok := r.Context().Value(notesKey{}).(*logNotes); ok {
		n.parts = append(n.parts, fmt.Sprintf(format, a...))
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		notes := &logNotes{}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), notesKey{}, notes)))
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		from := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			from += " via " + fwd // behind a reverse proxy: the reader's address
		}
		line := fmt.Sprintf("%s %s %d %s %s", r.Method, r.URL.Path, sw.status,
			time.Since(start).Round(time.Millisecond), from)
		if len(notes.parts) > 0 {
			line += " " + strings.Join(notes.parts, " ")
		}
		log.Print(line)
	})
}

func fmtTime(unix int64) string {
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}
