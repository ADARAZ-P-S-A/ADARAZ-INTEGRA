// Serwer odbic RAZ-RCP-1 - przyklad w Go (tylko biblioteka standardowa, Go 1.21+).
//
// Przyjmuje odbicia wysylane przez czytnik (Ustawienia > Integracje > Synchronizacja)
// i liczy z nich prosty czas pracy. Protokol i konfiguracja: ../README.md.
//
// Uruchomienie:
//
//	RCP_HASLO=tajne go run .
//
// W czytniku: adres http://<ip-komputera>:8080/odbicia, format http3, login rcp, haslo tajne.
// Raport czasu pracy: http://<ip-komputera>:8080/raport (ten sam login i haslo).
package main

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // strefy czasowe wkompilowane - dziala tez na Windows
	"unicode/utf8"
)

const (
	czasMin    = 1767225600 // 2026-01-01 UTC: wczesniej = zegar czytnika nieustawiony
	cialoMax   = 4096       // czytnik wysyla kilkadziesiat bajtow
	machineMax = 64
)

var (
	rfidRe     = regexp.MustCompile(`^[0-9a-f]{1,20}$`)
	czasRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)
	miesiacRe  = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	bialeZnaki = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")
)

type odbicie struct {
	epoch   int64
	rfid    string
	machine string
}

type serwer struct {
	login, haslo    string
	plikOdbic       string
	plikPracownikow string
	strefaCzytnika  *time.Location // UTC dla http2/http3, Europe/Warsaw dla formatow Skryba
	strefa          *time.Location // strefa raportu

	mu      sync.Mutex
	odbicia []odbicie
	znane   map[odbicie]bool
}

func env(nazwa, domyslna string) string {
	if v := os.Getenv(nazwa); v != "" {
		return v
	}
	return domyslna
}

// Bajty -> tekst UTF-8; kazdy bledny fragment (maksymalny niepelny ciag, jak
// Python errors="replace" i WHATWG) zamieniony na jedno U+FFFD.
func utf8Zastap(b []byte) string {
	var w strings.Builder
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r != utf8.RuneError || n > 1 {
			w.Write(b[:n])
			b = b[n:]
			continue
		}
		// zakres drugiego bajtu zalezy od pierwszego (Unicode, tabela 3-7)
		n, ile, od, do := 1, 0, byte(0x80), byte(0xBF)
		switch c := b[0]; {
		case c >= 0xC2 && c <= 0xDF:
			ile = 1
		case c == 0xE0:
			ile, od = 2, 0xA0
		case c == 0xED:
			ile, do = 2, 0x9F
		case c >= 0xE1 && c <= 0xEF:
			ile = 2
		case c == 0xF0:
			ile, od = 3, 0x90
		case c == 0xF4:
			ile, do = 3, 0x8F
		case c >= 0xF1 && c <= 0xF3:
			ile = 3
		}
		for n <= ile && n < len(b) && b[n] >= od && b[n] <= do {
			n, od, do = n+1, 0x80, 0xBF
		}
		w.WriteString("\uFFFD")
		b = b[n:]
	}
	return w.String()
}

func cyfraHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// Dekodowanie pola formularza jak Python unquote_plus(errors="replace"): '+' = spacja,
// %XX -> bajt, bledne % zostaje doslownie, bajty -> UTF-8 z U+FFFD.
func odkoduj(v string) string {
	var b []byte
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '+' {
			c = ' '
		} else if c == '%' && i+2 < len(v) {
			h, ok1 := cyfraHex(v[i+1])
			l, ok2 := cyfraHex(v[i+2])
			if ok1 && ok2 {
				c, i = h<<4|l, i+2
			}
		}
		b = append(b, c)
	}
	return utf8Zastap(b)
}

// Cialo formularza -> pola (pierwsza wartosc kazdej nazwy). Tylko separator '&',
// bez bledow - jak Python parse_qs(keep_blank_values=True, errors="replace").
func parsujFormularz(cialo []byte) map[string]string {
	pola := map[string]string{}
	for _, para := range strings.Split(string(cialo), "&") {
		if para == "" {
			continue
		}
		k, v, _ := strings.Cut(para, "=")
		k = odkodujPole(k)
		if _, jest := pola[k]; !jest {
			pola[k] = odkodujPole(v)
		}
	}
	return pola
}

// Jak w Pythonie: ciagi ASCII przez odkoduj (%XX), surowe bajty spoza ASCII
// osobno przez utf8Zastap - nie lacza sie z bajtami z %XX.
func odkodujPole(v string) string {
	var w strings.Builder
	for v != "" {
		i := 0
		for i < len(v) && v[i] < 0x80 {
			i++
		}
		w.WriteString(odkoduj(v[:i]))
		j := i
		for j < len(v) && v[j] >= 0x80 {
			j++
		}
		w.WriteString(utf8Zastap([]byte(v[i:j])))
		v = v[j:]
	}
	return w.String()
}

// Odbicia z pliku: linie "epoch<TAB>rfid<TAB>machineId" (LF albo CRLF).
func (s *serwer) wczytajOdbicia() error {
	dane, err := os.ReadFile(s.plikOdbic)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, linia := range strings.Split(utf8Zastap(dane), "\n") {
		pola := strings.Split(strings.TrimSuffix(linia, "\r"), "\t")
		if len(pola) != 3 {
			continue
		}
		epoch, err := strconv.ParseInt(pola[0], 10, 64)
		if err != nil {
			continue
		}
		o := odbicie{epoch, pola[1], pola[2]}
		if !s.znane[o] {
			s.znane[o] = true
			s.odbicia = append(s.odbicia, o)
		}
	}
	// Urwana ostatnia linia (awaria w trakcie zapisu): konczymy ja, zeby nowe
	// odbicie nie skleilo sie z nia w jedna linie.
	if len(dane) > 0 && dane[len(dane)-1] != '\n' {
		f, err := os.OpenFile(s.plikOdbic, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		if _, err = f.WriteString("\n"); err == nil {
			err = f.Sync()
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	}
	return nil
}

// Dopisuje odbicie do pliku i czeka, az bedzie na dysku - dopiero wtedy wolno
// odpowiedziec 200 (czytnik oznacza odbicie jako wyslane, a moze je usunac).
func (s *serwer) zapiszOdbicie(o odbicie) error {
	f, err := os.OpenFile(s.plikOdbic, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(f, "%d\t%s\t%s\n", o.epoch, o.rfid, o.machine); err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// "RRRR-MM-DD GG:MM:SS" w strefie czytnika -> epoch. Jesienna godzina podwojna:
// pozniejsze wystapienie (czas zimowy); wiosenna luka: przesuniecie do przodu.
func (s *serwer) parsujCzas(v string) (int64, bool) {
	if !czasRe.MatchString(v) {
		return 0, false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", v, s.strefaCzytnika)
	if err != nil {
		return 0, false
	}
	return t.Unix(), true
}

// Cialo formularza od czytnika -> (kod HTTP, odpowiedz JSON).
func (s *serwer) przyjmij(pola map[string]string) (int, string) {
	rfid := strings.ToLower(strings.TrimSpace(pola["rfidID"]))
	machine := bialeZnaki.Replace(pola["machineId"])
	if r := []rune(machine); len(r) > machineMax {
		machine = string(r[:machineMax])
	}

	// 400 = czytnik zdejmuje odbicie z kolejki i nie wysle go ponownie,
	// wiec tylko dla zadan, ktorych i tak nie da sie zapisac.
	if !rfidRe.MatchString(rfid) {
		return 400, `{"stored":false,"error":"rfidID"}`
	}
	epoch, ok := s.parsujCzas(strings.TrimSpace(pola["timestamp"]))
	if !ok {
		return 400, `{"stored":false,"error":"timestamp"}`
	}
	// Odbicie z nieustawionym zegarem: 200 z "stored":false i slowem "clock" -
	// czytnik zapisze w swoim logu, ze serwer go nie przyjal z powodu zegara.
	if epoch < czasMin {
		return 200, `{"stored":false,"reason":"clock"}`
	}

	o := odbicie{epoch, rfid, machine}
	s.mu.Lock()
	defer s.mu.Unlock()
	// To samo odbicie drugi raz (np. odpowiedz zginela po drodze) - 200,
	// zeby czytnik przestal je wysylac.
	if s.znane[o] {
		return 200, `{"stored":false,"reason":"duplicate"}`
	}
	if err := s.zapiszOdbicie(o); err != nil {
		log.Printf("blad zapisu %s: %v", s.plikOdbic, err)
		return 500, `{"stored":false,"error":"storage"}` // czytnik ponowi
	}
	s.znane[o] = true
	s.odbicia = append(s.odbicia, o)
	log.Printf("odbicie %s %s z %q", rfid, s.lokalny(epoch, "2006-01-02 15:04:05"), machine)
	return 200, `{"stored":true}`
}

// -- raport czasu pracy -------------------------------------------------------

// rfidID -> osoba z pliku "rfidID;Imie Nazwisko" (czytany przy kazdym raporcie).
// Kilka kart z ta sama nazwa = jedna osoba.
func (s *serwer) wczytajPracownikow() map[string]string {
	mapa := map[string]string{}
	dane, err := os.ReadFile(s.plikPracownikow)
	if err != nil {
		return mapa
	}
	tekst := strings.TrimPrefix(utf8Zastap(dane), "\uFEFF") // BOM z Notatnika
	for _, linia := range strings.Split(tekst, "\n") {
		linia = strings.TrimSpace(linia)
		if linia == "" || strings.HasPrefix(linia, "#") {
			continue
		}
		rfid, osoba, _ := strings.Cut(linia, ";")
		if osoba = strings.TrimSpace(osoba); osoba != "" {
			mapa[strings.ToLower(strings.TrimSpace(rfid))] = osoba
		}
	}
	return mapa
}

type zmiana struct {
	wej, wyj int64 // wyj == 0: brak wyjscia, osoba w pracy
}

// Odbicia -> zmiany per osoba jak w panelu czytnika: kolejne odbicia osoby
// parowane 1.+2., 3.+4., ... bez ciecia na polnocy (zmiana 22:00-06:00 to jedna
// zmiana). Nieparzyste ostatnie = osoba w pracy.
func zmiany(lista []odbicie, mapa map[string]string) map[string][]zmiana {
	per := map[string][]int64{}
	for _, o := range lista {
		osoba, ok := mapa[o.rfid]
		if !ok {
			osoba = "karta " + o.rfid
		}
		per[osoba] = append(per[osoba], o.epoch)
	}
	wynik := map[string][]zmiana{}
	for osoba, czasy := range per {
		sort.Slice(czasy, func(i, j int) bool { return czasy[i] < czasy[j] })
		for i := 0; i < len(czasy); i += 2 {
			z := zmiana{wej: czasy[i]}
			if i+1 < len(czasy) {
				z.wyj = czasy[i+1]
			}
			wynik[osoba] = append(wynik[osoba], z)
		}
	}
	return wynik
}

func (s *serwer) lokalny(epoch int64, uklad string) string {
	return time.Unix(epoch, 0).In(s.strefa).Format(uklad)
}

func godziny(sek int64) string {
	return fmt.Sprintf("%d:%02d", sek/3600, sek%3600/60)
}

func sasiedni(miesiac string, krok int) string {
	t, _ := time.Parse("2006-01", miesiac)
	return t.AddDate(0, krok, 0).Format("2006-01")
}

func (s *serwer) raport(miesiac string) string {
	s.mu.Lock()
	kopia := append([]odbicie(nil), s.odbicia...)
	s.mu.Unlock()
	per := zmiany(kopia, s.wczytajPracownikow())
	osoby := make([]string, 0, len(per))
	for o := range per {
		osoby = append(osoby, o)
	}
	sort.Strings(osoby)

	const dzien = "2006-01-02 15:04"
	var wPracy, sumy, wiersze strings.Builder
	for _, osoba := range osoby {
		e := html.EscapeString(osoba)
		var suma int64
		for _, z := range per[osoba] {
			if z.wyj == 0 {
				fmt.Fprintf(&wPracy, "<li>%s od %s</li>", e, s.lokalny(z.wej, dzien))
			}
			if s.lokalny(z.wej, "2006-01") != miesiac {
				continue
			}
			if z.wyj == 0 {
				fmt.Fprintf(&wiersze, "<tr><td>%s</td><td>%s</td><td>-</td><td>w pracy</td></tr>",
					e, s.lokalny(z.wej, dzien))
			} else {
				suma += z.wyj - z.wej
				fmt.Fprintf(&wiersze, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
					e, s.lokalny(z.wej, dzien), s.lokalny(z.wyj, dzien), godziny(z.wyj-z.wej))
			}
		}
		if suma > 0 {
			fmt.Fprintf(&sumy, "<tr><td>%s</td><td>%s</td></tr>", e, godziny(suma))
		}
	}
	lista := "<p>Nikt.</p>"
	if wPracy.Len() > 0 {
		lista = "<ul>" + wPracy.String() + "</ul>"
	}
	poprz, nast := sasiedni(miesiac, -1), sasiedni(miesiac, 1)
	return strings.NewReplacer(
		"{m}", miesiac, "{poprz}", poprz, "{nast}", nast, "{n}", strconv.Itoa(len(kopia)),
		"{w_pracy}", lista, "{sumy}", sumy.String(), "{zmiany}", wiersze.String(),
	).Replace(szablon)
}

const szablon = `<!doctype html>
<html lang="pl"><head><meta charset="utf-8"><title>Czas pracy {m}</title>
<style>body{font-family:'IBM Plex Sans',system-ui,sans-serif;margin:2em;color:#16151D;background:#F8F8FA}
h1,h2{font-family:Sora,system-ui,sans-serif;font-weight:600}a{color:#6A4CF6}
table{border-collapse:collapse;background:#fff}td,th{border:1px solid #E4E4EC;padding:4px 8px;text-align:left}
th{background:#EDE8FF}</style></head><body>
<h1>Czas pracy {m}</h1>
<p><a href="?miesiac={poprz}">&larr; {poprz}</a> | <a href="?miesiac={nast}">{nast} &rarr;</a>
| odbic w bazie: {n}</p>
<h2>Teraz w pracy</h2>
{w_pracy}
<h2>Suma w miesiacu</h2>
<table><tr><th>Osoba</th><th>Czas pracy</th></tr>{sumy}</table>
<h2>Zmiany</h2>
<table><tr><th>Osoba</th><th>Wejscie</th><th>Wyjscie</th><th>Czas</th></tr>{zmiany}</table>
</body></html>
`

// -- HTTP ---------------------------------------------------------------------

func odpowiedz(w http.ResponseWriter, kod int, typ, tresc string) {
	w.Header().Set("Content-Type", typ+"; charset=utf-8")
	w.WriteHeader(kod)
	io.WriteString(w, tresc)
}

// HTTP Basic z RCP_LOGIN / RCP_HASLO. Przy bledzie odpowiada 401 sam - czytnik
// przerywa wtedy cykl i ponawia pozniej.
func (s *serwer) zalogowany(w http.ResponseWriter, r *http.Request) bool {
	login, haslo, ok := r.BasicAuth()
	// ConstantTimeCompare: czas porownania nie zdradza, ile znakow sie zgadza
	if ok && subtle.ConstantTimeCompare([]byte(login), []byte(s.login))&
		subtle.ConstantTimeCompare([]byte(haslo), []byte(s.haslo)) == 1 {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="RCP"`)
	odpowiedz(w, 401, "application/json", `{"error":"unauthorized"}`)
	return false
}

func (s *serwer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Blad w obsludze zadania: 500 i serwer dziala dalej.
	defer func() {
		if e := recover(); e != nil {
			if e == http.ErrAbortHandler {
				panic(e)
			}
			log.Printf("blad obslugi %s %s: %v", r.Method, r.URL, e)
			odpowiedz(w, 500, "application/json", `{"error":"internal"}`)
		}
	}()
	switch r.Method {
	case http.MethodPost:
		// Formularz tylko z ciala - query string w adresie ignorowany.
		// Cialo czytane przed sprawdzeniem hasla - inaczej zostaloby w polaczeniu keep-alive.
		cialo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, cialoMax))
		if err != nil {
			kod := 400
			var zaDuze *http.MaxBytesError
			if errors.As(err, &zaDuze) {
				kod = 413
			}
			odpowiedz(w, kod, "application/json", `{"error":"body"}`)
			return
		}
		if !s.zalogowany(w, r) {
			return
		}
		kod, tresc := s.przyjmij(parsujFormularz(cialo))
		odpowiedz(w, kod, "application/json", tresc)
	case http.MethodGet:
		if r.URL.Path != "/" && r.URL.Path != "/raport" {
			odpowiedz(w, 404, "application/json", `{"error":"not found"}`)
			return
		}
		if !s.zalogowany(w, r) {
			return
		}
		miesiac := parsujFormularz([]byte(r.URL.RawQuery))["miesiac"]
		if !miesiacRe.MatchString(miesiac) {
			miesiac = s.lokalny(time.Now().Unix(), "2006-01")
		}
		odpowiedz(w, 200, "text/html", s.raport(miesiac))
	default:
		odpowiedz(w, 405, "application/json", `{"error":"method"}`)
	}
}

func main() {
	s := &serwer{
		login:           env("RCP_LOGIN", "rcp"),
		haslo:           os.Getenv("RCP_HASLO"),
		plikOdbic:       env("RCP_DANE", "odbicia.tsv"),
		plikPracownikow: env("RCP_PRACOWNICY", "pracownicy.csv"),
		znane:           map[odbicie]bool{},
	}
	if s.haslo == "" {
		log.Fatal("Ustaw haslo zmienna RCP_HASLO (to samo co w czytniku, pole Haslo).")
	}
	var err error
	if s.strefaCzytnika, err = time.LoadLocation(env("RCP_STREFA_CZYTNIKA", "UTC")); err != nil {
		log.Fatalf("Zla strefa RCP_STREFA_CZYTNIKA (np. UTC albo Europe/Warsaw): %v", err)
	}
	if s.strefa, err = time.LoadLocation(env("RCP_STREFA", "Europe/Warsaw")); err != nil {
		log.Fatalf("Zla strefa RCP_STREFA (np. Europe/Warsaw): %v", err)
	}
	if err := s.wczytajOdbicia(); err != nil {
		log.Fatal(err)
	}
	port := env("RCP_PORT", "8080")
	log.Printf("Serwer odbic na porcie %s, odbic w %s: %d", port, s.plikOdbic, len(s.odbicia))
	// Limity czasu: wolny albo zawieszony klient nie trzyma polaczenia w nieskonczonosc.
	srv := &http.Server{Addr: ":" + port, Handler: s, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
