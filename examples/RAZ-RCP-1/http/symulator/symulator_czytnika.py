#!/usr/bin/env python3
"""
Symulator czytnika RAZ-RCP-1 / RAZ-RCP-1 reader simulator (Python 3.9+, stdlib only).

Wysyla odbicia tak jak czytnik (POST application/x-www-form-urlencoded, HTTP Basic,
User-Agent RAZ-RCP/8, jedno polaczenie keep-alive na cykl) i sprawdza odpowiedzi serwera.

Sends punches exactly like the reader does and checks the server's answers.

    # test zgodnosci serwera / server conformance test
    python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --login rcp --haslo tajne --test

    # jedno odbicie teraz / a single punch now
    python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --login rcp --haslo tajne --karta 04a1b2c3

--format: http3 (UID hex, domyslny), http2 (UID dziesietnie), skryba8 / skryba10
(numer karty jak stara Skryba, czas polski - serwer z RCP_STREFA_CZYTNIKA=Europe/Warsaw).
"""

import argparse
import base64
import http.client
import os
import sys
import time
from datetime import datetime, timezone
from urllib.parse import quote_plus, urlsplit

try:
    from zoneinfo import ZoneInfo
except ImportError:                       # pragma: no cover
    ZoneInfo = None


class Czytnik:
    def __init__(self, url, login, haslo, fmt, machine, strefa="Europe/Warsaw"):
        u = urlsplit(url)
        if u.scheme not in ("http", "https"):
            sys.exit("adres musi zaczynac sie od http:// albo https://")
        self.https = u.scheme == "https"
        self.host, self.port = u.hostname, u.port
        self.sciezka = (u.path or "/") + ("?" + u.query if u.query else "")
        self.login, self.haslo, self.fmt, self.machine = login, haslo, fmt, machine
        self.strefa = strefa
        self.pol = None

    def polacz(self):
        klasa = http.client.HTTPSConnection if self.https else http.client.HTTPConnection
        self.pol = klasa(self.host, self.port, timeout=8)

    def zadanie(self, metoda, sciezka, cialo=None, haslo=None):
        """(kod, tresc). Jedno polaczenie na wiele zadan jak w czytniku; zerwane - nowe."""
        cred = "%s:%s" % (self.login, self.haslo if haslo is None else haslo)
        naglowki = {"Authorization": "Basic " + base64.b64encode(cred.encode()).decode(),
                    "User-Agent": "RAZ-RCP/8"}
        if cialo is not None:
            naglowki["Content-Type"] = "application/x-www-form-urlencoded"
        for proba in (1, 2):
            if self.pol is None:
                self.polacz()
            try:
                self.pol.request(metoda, sciezka, body=cialo, headers=naglowki)
                r = self.pol.getresponse()
                tresc = r.read().decode("utf-8", "replace")
                if r.getheader("Connection", "").lower() == "close":
                    self.pol.close()
                    self.pol = None
                return r.status, tresc
            except (http.client.RemoteDisconnected, ConnectionResetError, BrokenPipeError):
                self.pol.close()
                self.pol = None
                if proba == 2:
                    raise

    def cialo(self, uid_hex, ts, machine=None):
        """Cialo jak sync.c: rfidID=..&timestamp=..&machineId=.."""
        uid = bytes.fromhex(uid_hex)[:7]          # czytnik wysyla najwyzej 7 bajtow UID
        if self.fmt == "http3":
            rfid = uid.hex()
        elif self.fmt == "http2":
            rfid = str(int.from_bytes(uid, "big"))
        else:
            # Stara Skryba: pierwszy poziom antykolizji (karta 7 B: 0x88 + 3 bajty) i BCC
            cl1 = bytes([0x88]) + uid[:3] if len(uid) >= 7 else uid[:4]
            cl1 += bytes([cl1[0] ^ cl1[1] ^ cl1[2] ^ cl1[3]])
            rfid = str(int.from_bytes(cl1 if self.fmt == "skryba10" else cl1[:4], "big"))
        if self.fmt.startswith("skryba"):
            if ZoneInfo is None:
                sys.exit("formaty skryba wymagaja Pythona 3.9+ (zoneinfo)")
            czas = datetime.fromtimestamp(ts, ZoneInfo("Europe/Warsaw")).strftime("%Y-%m-%d %H:%M:%S")
        else:
            czas = datetime.fromtimestamp(ts, timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
        m = self.machine if machine is None else machine
        return "rfidID=%s&timestamp=%s&machineId=%s" % (rfid, quote_plus(czas), quote_plus(m)), rfid

    def odbicie(self, uid_hex, ts, machine=None, haslo=None):
        cialo, rfid = self.cialo(uid_hex, ts, machine)
        kod, tresc = self.zadanie("POST", self.sciezka, cialo, haslo)
        return kod, tresc, rfid


# -- test zgodnosci ----------------------------------------------------------

class Wynik:
    def __init__(self):
        self.bledy = 0

    def sprawdz(self, warunek, opis, szczegoly=""):
        print(("  OK   " if warunek else "  BLAD ") + opis + ("" if warunek else "  (%s)" % szczegoly))
        if not warunek:
            self.bledy += 1


def test(cz, adres_raportu):
    w = Wynik()
    uid = "04" + os.urandom(6).hex()          # losowa karta 7 B - test nie zderza sie z poprzednim
    teraz = int(time.time())
    print("Karta testowa %s, format %s" % (uid, cz.fmt))

    kod, tresc, _ = cz.odbicie(uid, teraz - 7200, haslo=cz.haslo + "-zle")
    w.sprawdz(kod == 401, "zle haslo -> 401", "kod %d" % kod)

    kod, tresc, rfid = cz.odbicie(uid, teraz - 7200)
    w.sprawdz(kod == 200 and '"stored":true' in tresc, "nowe odbicie -> 200 stored:true", "%d %s" % (kod, tresc))

    kod, tresc, _ = cz.odbicie(uid, teraz - 7200)
    w.sprawdz(kod == 200 and '"stored":false' in tresc and "clock" not in tresc,
              "to samo odbicie drugi raz -> 200 stored:false (duplikat)", "%d %s" % (kod, tresc))

    kod, tresc, _ = cz.odbicie(uid, 1000)     # 1970 - zegar czytnika nieustawiony
    w.sprawdz(kod == 200 and '"stored":false' in tresc and "clock" in tresc,
              "odbicie z 1970 r. -> 200 stored:false z 'clock'", "%d %s" % (kod, tresc))

    kod, tresc = cz.zadanie("POST", cz.sciezka, "timestamp=2026-10-28+08%3A00%3A00&machineId=x")
    w.sprawdz(kod == 400, "brak rfidID -> 400", "%d %s" % (kod, tresc))

    kod, tresc = cz.zadanie("POST", cz.sciezka, "rfidID=04a1b2c3&timestamp=wczoraj&machineId=x")
    w.sprawdz(kod == 400, "zly timestamp -> 400", "%d %s" % (kod, tresc))

    # machineId ze znakami, ktore rozbilyby formularz (czytnik koduje je jak tu)
    kod, tresc, _ = cz.odbicie(uid, teraz - 3600, machine="M1&rfidID=9%x y")
    w.sprawdz(kod == 200 and '"stored":true' in tresc, "wyjscie z machineId 'M1&rfidID=9%x y' -> 200 stored:true",
              "%d %s" % (kod, tresc))

    # Miesiac wejscia w strefie raportu (domyslnie Europe/Warsaw) - test tuz po polnocy
    # 1. dnia miesiaca daje zmiane z poprzedniego miesiaca.
    if ZoneInfo is not None:
        miesiac = datetime.fromtimestamp(teraz - 7200, ZoneInfo(cz.strefa)).strftime("%Y-%m")
        adres_raportu += ("&" if "?" in adres_raportu else "?") + "miesiac=" + miesiac
    kod, tresc = cz.zadanie("GET", adres_raportu)
    w.sprawdz(kod == 200 and ("karta " + rfid) in tresc, "GET %s -> 200 z karta testowa" % adres_raportu,
              "%d, karty %s w tresci: %s" % (kod, rfid, ("karta " + rfid) in tresc))
    w.sprawdz("1:00" in tresc, "raport liczy zmiane 1:00 (wejscie -2 h, wyjscie -1 h)")

    kod, tresc = cz.zadanie("GET", adres_raportu, haslo=cz.haslo + "-zle")
    w.sprawdz(kod == 401, "raport ze zlym haslem -> 401", "kod %d" % kod)

    print("Wynik: %s" % ("wszystko OK" if not w.bledy else "%d bledow" % w.bledy))
    return 1 if w.bledy else 0


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("url", help="adres serwera odbic jak w czytniku, np. http://127.0.0.1:8080/odbicia")
    p.add_argument("--login", default="rcp")
    p.add_argument("--haslo", required=True)
    p.add_argument("--format", choices=("http3", "http2", "skryba8", "skryba10"), default="http3")
    p.add_argument("--machine", default="RAZ-RCP-symulator", help="machineId")
    p.add_argument("--raport", default="/raport", help="sciezka raportu dla --test")
    p.add_argument("--strefa", default="Europe/Warsaw", help="RCP_STREFA serwera (miesiac raportu w --test)")
    g = p.add_mutually_exclusive_group(required=True)
    g.add_argument("--test", action="store_true", help="test zgodnosci serwera")
    g.add_argument("--karta", help="UID karty hex (np. 04a1b2c3) - jedno odbicie teraz")
    a = p.parse_args()

    cz = Czytnik(a.url, a.login, a.haslo, a.format, a.machine, a.strefa)
    if a.test:
        sys.exit(test(cz, a.raport))
    kod, tresc, rfid = cz.odbicie(a.karta, int(time.time()))
    print("rfidID=%s -> %d %s" % (rfid, kod, tresc.strip()))
    sys.exit(0 if kod == 200 else 1)


if __name__ == "__main__":
    main()
