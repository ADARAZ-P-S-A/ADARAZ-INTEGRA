#!/usr/bin/env python3
"""
Serwer odbic RAZ-RCP-1 - przyklad w Pythonie (tylko biblioteka standardowa, Python 3.9+).

Przyjmuje odbicia wysylane przez czytnik (Ustawienia > Integracje > Synchronizacja)
i liczy z nich prosty czas pracy. Protokol i konfiguracja: ../README.md.

Uruchomienie:
    RCP_HASLO=tajne python3 server.py

W czytniku: adres http://<ip-komputera>:8080/odbicia, format http3, login rcp, haslo tajne.
Raport czasu pracy: http://<ip-komputera>:8080/raport (ten sam login i haslo).

Na Windows strefy czasowe wymagaja pakietu tzdata: pip install tzdata
"""

import base64
import hmac
import html
import os
import re
import sys
import threading
import time
import traceback
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs
from zoneinfo import ZoneInfo

PORT = int(os.environ.get("RCP_PORT", "8080"))
LOGIN = os.environ.get("RCP_LOGIN", "rcp")
HASLO = os.environ.get("RCP_HASLO", "")
PLIK_ODBIC = os.environ.get("RCP_DANE", "odbicia.tsv")
PLIK_PRACOWNIKOW = os.environ.get("RCP_PRACOWNICY", "pracownicy.csv")


def strefa(zmienna, domyslna):
    """Strefa ze zmiennej srodowiska - zla nazwa konczy program od razu przy starcie."""
    nazwa = os.environ.get(zmienna, domyslna)
    try:
        return ZoneInfo(nazwa)
    except Exception:
        sys.exit("Nieznana strefa czasowa %s=%r (np. UTC, Europe/Warsaw)." % (zmienna, nazwa))


# Strefa pola timestamp: UTC dla formatow http2/http3, Europe/Warsaw dla formatow Skryba.
STREFA_CZYTNIKA = strefa("RCP_STREFA_CZYTNIKA", "UTC")
# Strefa, w ktorej raport pokazuje godziny i dzieli odbicia na miesiace.
STREFA = strefa("RCP_STREFA", "Europe/Warsaw")

CZAS_MIN = 1767225600            # 2026-01-01 UTC: wczesniej = zegar czytnika nieustawiony
CIALO_MAX = 4096                 # czytnik wysyla kilkadziesiat bajtow
RFID = re.compile(r"[0-9a-f]{1,20}")
MACHINE_MAX = 64

blokada = threading.Lock()
odbicia = []                     # [(epoch, rfid, machine)] w kolejnosci przyjecia
znane = set()                    # te same krotki - do wykrywania duplikatow


def wczytaj_odbicia():
    """Odbicia z pliku: linie 'epoch<TAB>rfid<TAB>machineId' (LF albo CRLF)."""
    try:
        # Ostatnia linia bez '\n' (awaria w trakcie zapisu) - konczymy ja, zeby
        # nastepne odbicie nie dokleilo sie do niej.
        with open(PLIK_ODBIC, "rb+") as f:
            if f.seek(0, os.SEEK_END) > 0 and f.seek(-1, os.SEEK_END) >= 0 and f.read(1) != b"\n":
                f.write(b"\n")
                f.flush()
                os.fsync(f.fileno())
        # Zle bajty -> U+FFFD zamiast bledu; newline="\n" - linie tylko po '\n'.
        with open(PLIK_ODBIC, encoding="utf-8", errors="replace", newline="\n") as f:
            for linia in f:
                linia = linia.rstrip("\n")
                if linia.endswith("\r"):
                    linia = linia[:-1]
                pola = linia.split("\t")
                if len(pola) != 3 or not re.fullmatch(r"[0-9]+", pola[0]):
                    continue
                o = (int(pola[0]), pola[1], pola[2])
                if o not in znane:
                    znane.add(o)
                    odbicia.append(o)
    except FileNotFoundError:
        pass


def zapisz_odbicie(o):
    """Dopisuje odbicie do pliku i czeka, az bedzie na dysku - dopiero wtedy wolno
    odpowiedziec 200 (czytnik oznacza odbicie jako wyslane, a moze je usunac)."""
    # newline="\n": zawsze LF, takze na Windows
    with open(PLIK_ODBIC, "a", encoding="utf-8", newline="\n") as f:
        f.write("%d\t%s\t%s\n" % o)
        f.flush()
        os.fsync(f.fileno())


def parsuj_czas(s):
    """'RRRR-MM-DD GG:MM:SS' w strefie czytnika -> epoch, None przy zlym zapisie."""
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}", s, re.ASCII):
        return None
    try:
        dt = datetime.strptime(s, "%Y-%m-%d %H:%M:%S")
    except ValueError:
        return None
    # Zmiana czasu: godzina podwojna jesienia -> pozniejsza (czas zimowy),
    # nieistniejaca wiosna -> z przesunieciem sprzed zmiany (02:30 = 03:30 CEST).
    return int(max(dt.replace(tzinfo=STREFA_CZYTNIKA, fold=0).timestamp(),
                   dt.replace(tzinfo=STREFA_CZYTNIKA, fold=1).timestamp()))


def pole(pola, nazwa):
    """Pierwsza wartosc pola: bajty z %XX jako UTF-8, zle sekwencje -> U+FFFD."""
    return (pola.get(nazwa) or [""])[0].encode("latin-1").decode("utf-8", "replace")


def przyjmij(cialo):
    """Cialo formularza (bajty) od czytnika -> (kod HTTP, odpowiedz JSON)."""
    # latin-1: kazdy bajt (tez z %XX) = jeden znak; UTF-8 dekoduje dopiero pole()
    pola = parse_qs(cialo.decode("latin-1"), keep_blank_values=True, encoding="latin-1")
    rfid = pole(pola, "rfidID").strip().lower()
    czas = pole(pola, "timestamp").strip()
    machine = pole(pola, "machineId")
    machine = re.sub(r"[\t\r\n]", " ", machine)[:MACHINE_MAX]

    # 400 = czytnik zdejmuje odbicie z kolejki i nie wysle go ponownie,
    # wiec tylko dla zadan, ktorych i tak nie da sie zapisac.
    if not RFID.fullmatch(rfid):
        return 400, '{"stored":false,"error":"rfidID"}'
    epoch = parsuj_czas(czas)
    if epoch is None:
        return 400, '{"stored":false,"error":"timestamp"}'
    # Odbicie z nieustawionym zegarem: 200 z "stored":false i slowem "clock" -
    # czytnik zapisze w swoim logu, ze serwer go nie przyjal z powodu zegara.
    if epoch < CZAS_MIN:
        return 200, '{"stored":false,"reason":"clock"}'

    o = (epoch, rfid, machine)
    with blokada:
        # To samo odbicie drugi raz (np. odpowiedz zginela po drodze) - 200,
        # zeby czytnik przestal je wysylac.
        if o in znane:
            return 200, '{"stored":false,"reason":"duplicate"}'
        try:
            zapisz_odbicie(o)
        except OSError as e:
            print("blad zapisu %s: %s" % (PLIK_ODBIC, e), file=sys.stderr)
            return 500, '{"stored":false,"error":"storage"}'    # czytnik ponowi
        znane.add(o)
        odbicia.append(o)
    print("odbicie %s %s z %r" % (rfid, lokalny(epoch, "%Y-%m-%d %H:%M:%S"), machine), flush=True)
    return 200, '{"stored":true}'


# -- raport czasu pracy ------------------------------------------------------

def wczytaj_pracownikow():
    """rfidID -> osoba z pliku 'rfidID;Imie Nazwisko' (czytany przy kazdym raporcie).
    Kilka kart z ta sama nazwa = jedna osoba. utf-8-sig zdejmuje BOM, zle bajty
    (np. plik w Windows-1250) -> U+FFFD zamiast bledu."""
    mapa = {}
    try:
        with open(PLIK_PRACOWNIKOW, encoding="utf-8-sig", errors="replace", newline="\n") as f:
            for linia in f:
                linia = linia.strip()
                if not linia or linia.startswith("#"):
                    continue
                rfid, _, osoba = linia.partition(";")
                if osoba.strip():
                    mapa[rfid.strip().lower()] = osoba.strip()
    except FileNotFoundError:
        pass
    return mapa


def zmiany(lista, mapa):
    """Odbicia -> {osoba: [(wejscie, wyjscie albo None)]} jak w panelu czytnika:
    kolejne odbicia osoby parowane 1.+2., 3.+4., ... bez ciecia na polnocy
    (zmiana 22:00-06:00 to jedna zmiana). Nieparzyste ostatnie = osoba w pracy."""
    per = {}
    for epoch, rfid, _ in lista:
        per.setdefault(mapa.get(rfid, "karta " + rfid), []).append(epoch)
    wynik = {}
    for osoba, czasy in per.items():
        czasy.sort()
        wynik[osoba] = [(czasy[i], czasy[i + 1] if i + 1 < len(czasy) else None)
                        for i in range(0, len(czasy), 2)]
    return wynik


def lokalny(epoch, fmt="%Y-%m-%d %H:%M"):
    return datetime.fromtimestamp(epoch, timezone.utc).astimezone(STREFA).strftime(fmt)


def godziny(s):
    return "%d:%02d" % (s // 3600, s % 3600 // 60)


def sasiedni(miesiac, krok):
    r, m = int(miesiac[:4]), int(miesiac[5:]) + krok
    if m < 1:
        r, m = r - 1, 12
    elif m > 12:
        r, m = r + 1, 1
    return "%04d-%02d" % (r, m)


def raport(miesiac):
    with blokada:
        kopia = list(odbicia)
    per = zmiany(kopia, wczytaj_pracownikow())
    e = html.escape
    w_pracy, sumy, wiersze = [], [], []
    for osoba in sorted(per):
        suma = 0
        for wej, wyj in per[osoba]:
            if wyj is None:
                w_pracy.append("<li>%s od %s</li>" % (e(osoba), lokalny(wej)))
            if lokalny(wej, "%Y-%m") != miesiac:
                continue
            if wyj is None:
                wiersze.append("<tr><td>%s</td><td>%s</td><td>-</td><td>w pracy</td></tr>"
                               % (e(osoba), lokalny(wej)))
            else:
                suma += wyj - wej
                wiersze.append("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>"
                               % (e(osoba), lokalny(wej), lokalny(wyj), godziny(wyj - wej)))
        if suma:
            sumy.append("<tr><td>%s</td><td>%s</td></tr>" % (e(osoba), godziny(suma)))
    return SZABLON % {
        "m": miesiac, "poprz": sasiedni(miesiac, -1), "nast": sasiedni(miesiac, 1),
        "n": len(kopia),
        "w_pracy": "<ul>%s</ul>" % "".join(w_pracy) if w_pracy else "<p>Nikt.</p>",
        "sumy": "".join(sumy), "zmiany": "".join(wiersze),
    }


SZABLON = """<!doctype html>
<html lang="pl"><head><meta charset="utf-8"><title>Czas pracy %(m)s</title>
<style>body{font-family:'IBM Plex Sans',system-ui,sans-serif;margin:2em;color:#16151D;background:#F8F8FA}
h1,h2{font-family:Sora,system-ui,sans-serif;font-weight:600}a{color:#6A4CF6}
table{border-collapse:collapse;background:#fff}td,th{border:1px solid #E4E4EC;padding:4px 8px;text-align:left}
th{background:#EDE8FF}</style></head><body>
<h1>Czas pracy %(m)s</h1>
<p><a href="?miesiac=%(poprz)s">&larr; %(poprz)s</a> | <a href="?miesiac=%(nast)s">%(nast)s &rarr;</a>
| odbic w bazie: %(n)d</p>
<h2>Teraz w pracy</h2>
%(w_pracy)s
<h2>Suma w miesiacu</h2>
<table><tr><th>Osoba</th><th>Czas pracy</th></tr>%(sumy)s</table>
<h2>Zmiany</h2>
<table><tr><th>Osoba</th><th>Wejscie</th><th>Wyjscie</th><th>Czas</th></tr>%(zmiany)s</table>
</body></html>
"""


# -- HTTP --------------------------------------------------------------------

def bezpiecznie(obsluga):
    """Wyjatek w obsludze zadania -> 500, serwer dziala dalej."""
    def opakowana(self):
        self.wyslano = False
        try:
            obsluga(self)
        except (TimeoutError, ConnectionError):
            raise                    # klient zawieszony albo rozlaczony - nie ma komu odpowiadac
        except Exception:
            traceback.print_exc()
            self.close_connection = True
            if not self.wyslano:
                try:
                    self.odpowiedz(500, '{"error":"internal"}')
                except Exception:
                    pass
    return opakowana


class Handler(BaseHTTPRequestHandler):
    # keep-alive: czytnik wysyla cala paczke odbic jednym polaczeniem
    protocol_version = "HTTP/1.1"
    # zawieszony klient: po 30 s bez danych polaczenie jest zamykane
    timeout = 30

    def odpowiedz(self, kod, tresc, typ="application/json", naglowki=()):
        dane = tresc.encode("utf-8")
        self.wyslano = True
        self.send_response(kod)
        self.send_header("Content-Type", typ + "; charset=utf-8")
        self.send_header("Content-Length", str(len(dane)))
        for k, v in naglowki:
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(dane)

    def zalogowany(self):
        """HTTP Basic z RCP_LOGIN / RCP_HASLO. Przy bledzie odpowiada 401 sam -
        czytnik przerywa wtedy cykl i ponawia pozniej."""
        nagl = self.headers.get("Authorization", "")
        if nagl.startswith("Basic "):
            try:
                login, _, haslo = base64.b64decode(nagl[6:], validate=True).decode("utf-8").partition(":")
            except ValueError:
                login = haslo = ""
            # compare_digest: czas porownania nie zdradza, ile znakow sie zgadza
            if hmac.compare_digest(login.encode(), LOGIN.encode()) & \
                    hmac.compare_digest(haslo.encode(), HASLO.encode()):
                return True
        self.odpowiedz(401, '{"error":"unauthorized"}',
                       naglowki=[("WWW-Authenticate", 'Basic realm="RCP"')])
        return False

    @bezpiecznie
    def do_POST(self):
        # Cialo czytane przed sprawdzeniem hasla - inaczej zostaloby w polaczeniu keep-alive.
        # Formularz tylko z ciala - query string w adresie jest pomijany.
        try:
            dlugosc = int(self.headers.get("Content-Length") or 0)
        except ValueError:
            dlugosc = -1
        if not 0 <= dlugosc <= CIALO_MAX:
            self.close_connection = True
            return self.odpowiedz(413, '{"error":"body"}')
        cialo = self.rfile.read(dlugosc)
        if self.zalogowany():
            self.odpowiedz(*przyjmij(cialo))

    @bezpiecznie
    def do_GET(self):
        sciezka, _, query = self.path.partition("?")
        if sciezka not in ("/", "/raport"):
            return self.odpowiedz(404, '{"error":"not found"}')
        if not self.zalogowany():
            return
        # zly query string = biezacy miesiac, nigdy 400
        miesiac = (parse_qs(query).get("miesiac") or [""])[0]
        if not re.fullmatch(r"\d{4}-(0[1-9]|1[0-2])", miesiac, re.ASCII):
            miesiac = lokalny(time.time(), "%Y-%m")
        self.odpowiedz(200, raport(miesiac), typ="text/html")


def main():
    if not HASLO:
        sys.exit("Ustaw haslo zmienna RCP_HASLO (to samo co w czytniku, pole Haslo).")
    # log z U+FFFD / polskimi znakami nie wywroci zapisu do konsoli cp1250
    for strumien in (sys.stdout, sys.stderr):
        if hasattr(strumien, "reconfigure"):
            strumien.reconfigure(errors="backslashreplace")
    wczytaj_odbicia()
    print("Serwer odbic na porcie %d, odbic w %s: %d" % (PORT, PLIK_ODBIC, len(odbicia)), flush=True)
    ThreadingHTTPServer(("", PORT), Handler).serve_forever()


if __name__ == "__main__":
    main()
