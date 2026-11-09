#!/usr/bin/env node
// Serwer odbic RAZ-RCP-1 - przyklad w Node.js (bez zaleznosci, Node 18+).
//
// Przyjmuje odbicia wysylane przez czytnik (Ustawienia > Integracje > Synchronizacja)
// i liczy z nich prosty czas pracy. Protokol i konfiguracja: ../README.md.
//
// Uruchomienie:
//   RCP_HASLO=tajne node server.js
//
// W czytniku: adres http://<ip-komputera>:8080/odbicia, format http3, login rcp, haslo tajne.
// Raport czasu pracy: http://<ip-komputera>:8080/raport (ten sam login i haslo).
'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');

const PORT = Number(process.env.RCP_PORT || 8080);
const LOGIN = process.env.RCP_LOGIN || 'rcp';
const HASLO = process.env.RCP_HASLO || '';
const PLIK_ODBIC = process.env.RCP_DANE || 'odbicia.tsv';
const PLIK_PRACOWNIKOW = process.env.RCP_PRACOWNICY || 'pracownicy.csv';

// Strefa ze zmiennej srodowiska - zla nazwa konczy program od razu przy starcie.
function strefa(zmienna, domyslna) {
  const nazwa = process.env[zmienna] || domyslna;
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: nazwa });
  } catch {
    console.error(`Nieznana strefa czasowa ${zmienna}=${JSON.stringify(nazwa)} (np. UTC, Europe/Warsaw).`);
    process.exit(1);
  }
  return nazwa;
}

// Strefa pola timestamp: UTC dla formatow http2/http3, Europe/Warsaw dla formatow Skryba.
const STREFA_CZYTNIKA = strefa('RCP_STREFA_CZYTNIKA', 'UTC');
// Strefa, w ktorej raport pokazuje godziny i dzieli odbicia na miesiace.
const STREFA = strefa('RCP_STREFA', 'Europe/Warsaw');

const CZAS_MIN = 1767225600;     // 2026-01-01 UTC: wczesniej = zegar czytnika nieustawiony
const CIALO_MAX = 4096;          // czytnik wysyla kilkadziesiat bajtow
const MACHINE_MAX = 64;
const DOBA = 86400000;

const odbicia = [];              // [{ epoch, rfid, machine }] w kolejnosci przyjecia
const znane = new Set();         // klucze "epoch\trfid\tmachine" - do wykrywania duplikatow

const klucz = (o) => `${o.epoch}\t${o.rfid}\t${o.machine}`;

// Odbicia z pliku: linie "epoch<TAB>rfid<TAB>machineId" (LF albo CRLF).
function wczytajOdbicia() {
  let dane;
  try {
    dane = fs.readFileSync(PLIK_ODBIC);
  } catch (e) {
    if (e.code === 'ENOENT') return;
    throw e;
  }
  // Ostatnia linia bez '\n' (awaria w trakcie zapisu) - konczymy ja, zeby
  // nastepne odbicie nie dokleilo sie do niej.
  if (dane.length && dane[dane.length - 1] !== 0x0a) dopisz('\n');
  // Zle bajty -> U+FFFD zamiast bledu.
  for (let linia of dane.toString('utf8').split('\n')) {
    if (linia.endsWith('\r')) linia = linia.slice(0, -1);
    const pola = linia.split('\t');
    if (pola.length !== 3 || !/^\d+$/.test(pola[0])) continue;
    const o = { epoch: Number(pola[0]), rfid: pola[1], machine: pola[2] };
    if (!znane.has(klucz(o))) {
      znane.add(klucz(o));
      odbicia.push(o);
    }
  }
}

// Dopisuje odbicie do pliku i czeka, az bedzie na dysku - dopiero wtedy wolno
// odpowiedziec 200 (czytnik oznacza odbicie jako wyslane, a moze je usunac).
// Synchronicznie: przy jednym watku JS to zarazem blokada przed rownoleglym zapisem.
function zapiszOdbicie(o) {
  dopisz(klucz(o) + '\n');
}

function dopisz(tekst) {
  const fd = fs.openSync(PLIK_ODBIC, 'a', 0o600);
  try {
    fs.writeSync(fd, tekst);
    fs.fsyncSync(fd);
  } finally {
    fs.closeSync(fd);
  }
}

// Czesci daty w strefie: { year, month, day, hour, minute, second } jako liczby.
function czesci(ms, strefa) {
  const f = new Intl.DateTimeFormat('en-US', {
    timeZone: strefa, hourCycle: 'h23',
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  });
  const p = {};
  for (const { type, value } of f.formatToParts(new Date(ms))) p[type] = Number(value);
  return p;
}

// Date.UTC, ale lata 0-99 bez zamiany na 1900-1999.
function utc(r, mi, d, g, min, sek) {
  const t = new Date(0);
  t.setUTCFullYear(r, mi - 1, d);
  return t.setUTCHours(g, min, sek);
}

// Przesuniecie strefy czytnika w chwili ms (w ms).
function przesuniecie(ms) {
  const p = czesci(ms, STREFA_CZYTNIKA);
  return utc(p.year, p.month, p.day, p.hour, p.minute, p.second) - ms;
}

// "RRRR-MM-DD GG:MM:SS" w strefie czytnika -> epoch, null przy zlym zapisie.
function parsujCzas(s) {
  const m = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2}):(\d{2})$/.exec(s);
  if (!m) return null;
  const [r, mi, d, g, min, sek] = m.slice(1).map(Number);
  const jakUtc = utc(r, mi, d, g, min, sek);
  const p0 = czesci(jakUtc, 'UTC');
  if (p0.year !== r || p0.month !== mi || p0.day !== d || p0.hour !== g || p0.minute !== min ||
      p0.second !== sek) return null;   // np. 2026-02-30 albo 25:00
  // Przesuniecie dobe przed i dobe po - przy zmianie czasu sa rozne. Godzina podwojna
  // jesienia -> pozniejsza (czas zimowy), nieistniejaca wiosna -> z przesunieciem
  // sprzed zmiany (02:30 = 03:30 CEST).
  const przed = przesuniecie(jakUtc - DOBA);
  const kandydaci = [przed, przesuniecie(jakUtc + DOBA)].map((o) => jakUtc - o)
    .filter((ms) => przesuniecie(ms) === jakUtc - ms);
  const ms = kandydaci.length ? Math.max(...kandydaci) : jakUtc - przed;
  return Math.floor(ms / 1000);
}

// %XX -> bajt, '+' -> spacja; zly %XX zostaje doslownie.
function zProcentow(b) {
  const wynik = Buffer.alloc(b.length);
  let n = 0;
  for (let i = 0; i < b.length; i++) {
    if (b[i] === 0x25 && /^[0-9A-Fa-f]{2}$/.test(b.toString('latin1', i + 1, i + 3))) {
      wynik[n++] = parseInt(b.toString('latin1', i + 1, i + 3), 16);
      i += 2;
    } else {
      wynik[n++] = b[i] === 0x2b ? 0x20 : b[i];
    }
  }
  return wynik.subarray(0, n);
}

// Cialo formularza (bajty) -> Map(nazwa -> pierwsza wartosc). Wartosc to bajty
// z %XX jako UTF-8, zle sekwencje -> U+FFFD (jak parse_qs w przykladzie w Pythonie).
function formularz(cialo) {
  const pola = new Map();
  for (let start = 0; start <= cialo.length;) {
    let koniec = cialo.indexOf(0x26, start);   // '&'
    if (koniec < 0) koniec = cialo.length;
    const para = cialo.subarray(start, koniec);
    start = koniec + 1;
    if (!para.length) continue;
    let i = para.indexOf(0x3d);                // '='
    if (i < 0) i = para.length;
    const nazwa = zProcentow(para.subarray(0, i)).toString('latin1');
    if (!pola.has(nazwa)) pola.set(nazwa, zProcentow(para.subarray(i + 1)).toString('utf8'));
  }
  return pola;
}

// Cialo formularza (bajty) od czytnika -> [kod HTTP, odpowiedz JSON].
function przyjmij(cialo) {
  const pola = formularz(cialo);
  const rfid = (pola.get('rfidID') || '').trim().toLowerCase();
  const machine = [...(pola.get('machineId') || '').replace(/[\t\r\n]/g, ' ')]
    .slice(0, MACHINE_MAX).join('');

  // 400 = czytnik zdejmuje odbicie z kolejki i nie wysle go ponownie,
  // wiec tylko dla zadan, ktorych i tak nie da sie zapisac.
  if (!/^[0-9a-f]{1,20}$/.test(rfid)) return [400, '{"stored":false,"error":"rfidID"}'];
  const epoch = parsujCzas((pola.get('timestamp') || '').trim());
  if (epoch === null) return [400, '{"stored":false,"error":"timestamp"}'];
  // Odbicie z nieustawionym zegarem: 200 z "stored":false i slowem "clock" -
  // czytnik zapisze w swoim logu, ze serwer go nie przyjal z powodu zegara.
  if (epoch < CZAS_MIN) return [200, '{"stored":false,"reason":"clock"}'];

  const o = { epoch, rfid, machine };
  // To samo odbicie drugi raz (np. odpowiedz zginela po drodze) - 200,
  // zeby czytnik przestal je wysylac.
  if (znane.has(klucz(o))) return [200, '{"stored":false,"reason":"duplicate"}'];
  try {
    zapiszOdbicie(o);
  } catch (e) {
    console.error(`blad zapisu ${PLIK_ODBIC}: ${e.message}`);
    return [500, '{"stored":false,"error":"storage"}'];    // czytnik ponowi
  }
  znane.add(klucz(o));
  odbicia.push(o);
  console.log(`odbicie ${rfid} ${lokalny(epoch, true)} z ${JSON.stringify(machine)}`);
  return [200, '{"stored":true}'];
}

// -- raport czasu pracy -------------------------------------------------------

// rfidID -> osoba z pliku "rfidID;Imie Nazwisko" (czytany przy kazdym raporcie).
// Kilka kart z ta sama nazwa = jedna osoba. Zle bajty (np. plik w Windows-1250)
// -> U+FFFD zamiast bledu, BOM na poczatku pliku pomijany.
function wczytajPracownikow() {
  const mapa = new Map();
  let dane;
  try {
    dane = fs.readFileSync(PLIK_PRACOWNIKOW, 'utf8');
  } catch {
    return mapa;
  }
  if (dane.startsWith('\uFEFF')) dane = dane.slice(1);
  for (let linia of dane.split('\n')) {
    linia = linia.trim();
    if (!linia || linia.startsWith('#')) continue;
    const i = linia.indexOf(';');
    const osoba = i < 0 ? '' : linia.slice(i + 1).trim();
    if (osoba) mapa.set(linia.slice(0, i).trim().toLowerCase(), osoba);
  }
  return mapa;
}

// Odbicia -> Map(osoba -> [{ wej, wyj albo null }]) jak w panelu czytnika:
// kolejne odbicia osoby parowane 1.+2., 3.+4., ... bez ciecia na polnocy
// (zmiana 22:00-06:00 to jedna zmiana). Nieparzyste ostatnie = osoba w pracy.
function zmiany(lista, mapa) {
  const per = new Map();
  for (const o of lista) {
    const osoba = mapa.get(o.rfid) || `karta ${o.rfid}`;
    if (!per.has(osoba)) per.set(osoba, []);
    per.get(osoba).push(o.epoch);
  }
  const wynik = new Map();
  for (const [osoba, czasy] of per) {
    czasy.sort((a, b) => a - b);
    const z = [];
    for (let i = 0; i < czasy.length; i += 2) z.push({ wej: czasy[i], wyj: czasy[i + 1] ?? null });
    wynik.set(osoba, z);
  }
  return wynik;
}

const dwa = (n) => String(n).padStart(2, '0');

// "RRRR-MM-DD GG:MM" (z sekundami: "...:SS") w strefie raportu.
function lokalny(epoch, sekundy = false) {
  const p = czesci(epoch * 1000, STREFA);
  const s = `${p.year}-${dwa(p.month)}-${dwa(p.day)} ${dwa(p.hour)}:${dwa(p.minute)}`;
  return sekundy ? `${s}:${dwa(p.second)}` : s;
}

const godziny = (s) => `${Math.floor(s / 3600)}:${dwa(Math.floor((s % 3600) / 60))}`;

function sasiedni(miesiac, krok) {
  let r = Number(miesiac.slice(0, 4));
  let m = Number(miesiac.slice(5)) + krok;
  if (m < 1) { r--; m = 12; } else if (m > 12) { r++; m = 1; }
  return `${r}-${dwa(m)}`;
}

const ENCJE = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
const esc = (s) => s.replace(/[&<>"']/g, (c) => ENCJE[c]);

function raport(miesiac) {
  const per = zmiany(odbicia, wczytajPracownikow());
  const wPracy = [];
  const sumy = [];
  const wiersze = [];
  for (const osoba of [...per.keys()].sort()) {
    const e = esc(osoba);
    let suma = 0;
    for (const { wej, wyj } of per.get(osoba)) {
      if (wyj === null) wPracy.push(`<li>${e} od ${lokalny(wej)}</li>`);
      if (lokalny(wej).slice(0, 7) !== miesiac) continue;
      if (wyj === null) {
        wiersze.push(`<tr><td>${e}</td><td>${lokalny(wej)}</td><td>-</td><td>w pracy</td></tr>`);
      } else {
        suma += wyj - wej;
        wiersze.push(`<tr><td>${e}</td><td>${lokalny(wej)}</td><td>${lokalny(wyj)}</td>` +
                     `<td>${godziny(wyj - wej)}</td></tr>`);
      }
    }
    if (suma) sumy.push(`<tr><td>${e}</td><td>${godziny(suma)}</td></tr>`);
  }
  const poprz = sasiedni(miesiac, -1);
  const nast = sasiedni(miesiac, 1);
  return `<!doctype html>
<html lang="pl"><head><meta charset="utf-8"><title>Czas pracy ${miesiac}</title>
<style>body{font-family:'IBM Plex Sans',system-ui,sans-serif;margin:2em;color:#16151D;background:#F8F8FA}
h1,h2{font-family:Sora,system-ui,sans-serif;font-weight:600}a{color:#6A4CF6}
table{border-collapse:collapse;background:#fff}td,th{border:1px solid #E4E4EC;padding:4px 8px;text-align:left}
th{background:#EDE8FF}</style></head><body>
<h1>Czas pracy ${miesiac}</h1>
<p><a href="?miesiac=${poprz}">&larr; ${poprz}</a> | <a href="?miesiac=${nast}">${nast} &rarr;</a>
| odbic w bazie: ${odbicia.length}</p>
<h2>Teraz w pracy</h2>
${wPracy.length ? `<ul>${wPracy.join('')}</ul>` : '<p>Nikt.</p>'}
<h2>Suma w miesiacu</h2>
<table><tr><th>Osoba</th><th>Czas pracy</th></tr>${sumy.join('')}</table>
<h2>Zmiany</h2>
<table><tr><th>Osoba</th><th>Wejscie</th><th>Wyjscie</th><th>Czas</th></tr>${wiersze.join('')}</table>
</body></html>
`;
}

// -- HTTP ---------------------------------------------------------------------

function odpowiedz(res, kod, tresc, typ = 'application/json', naglowki = {}) {
  res.writeHead(kod, { 'Content-Type': `${typ}; charset=utf-8`, ...naglowki });
  res.end(tresc);
}

// timingSafeEqual: czas porownania nie zdradza, ile znakow sie zgadza
// (skroty SHA-256 - timingSafeEqual wymaga rownej dlugosci).
function rowne(a, b) {
  const h = (s) => crypto.createHash('sha256').update(s).digest();
  return crypto.timingSafeEqual(h(a), h(b));
}

// HTTP Basic z RCP_LOGIN / RCP_HASLO. Przy bledzie odpowiada 401 sam - czytnik
// przerywa wtedy cykl i ponawia pozniej.
function zalogowany(req, res) {
  const m = /^Basic ([A-Za-z0-9+/=]+)$/.exec(req.headers.authorization || '');
  if (m) {
    const cred = Buffer.from(m[1], 'base64').toString('utf8');
    const i = cred.indexOf(':');
    if (i >= 0 && rowne(cred.slice(0, i), LOGIN) & rowne(cred.slice(i + 1), HASLO)) return true;
  }
  odpowiedz(res, 401, '{"error":"unauthorized"}', 'application/json',
            { 'WWW-Authenticate': 'Basic realm="RCP"' });
  return false;
}

// Wyjatek w obsludze zadania -> 500, serwer dziala dalej.
function bezpiecznie(res, obsluga) {
  try {
    obsluga();
  } catch (e) {
    console.error(e);
    if (res.headersSent) return res.destroy();
    res.setHeader('Connection', 'close');
    odpowiedz(res, 500, '{"error":"internal"}');
  }
}

function obsluz(req, res) {
  // Bez new URL - sciezka typu "//" rzucalaby wyjatek.
  const [sciezka, ...reszta] = req.url.split('?');
  req.on('error', () => {});     // klient rozlaczony w trakcie wysylania ciala
  if (req.method === 'POST') {
    // Cialo czytane przed sprawdzeniem hasla - inaczej zostaloby w polaczeniu keep-alive.
    // Formularz tylko z ciala - query string w adresie jest pomijany.
    const kawalki = [];
    let dlugosc = 0;
    req.on('data', (k) => bezpiecznie(res, () => {
      dlugosc += k.length;
      if (dlugosc > CIALO_MAX) {
        if (res.headersSent) return;
        res.setHeader('Connection', 'close');
        odpowiedz(res, 413, '{"error":"body"}');
        req.destroy();
      } else {
        kawalki.push(k);
      }
    }));
    req.on('end', () => bezpiecznie(res, () => {
      if (dlugosc > CIALO_MAX || !zalogowany(req, res)) return;
      const [kod, tresc] = przyjmij(Buffer.concat(kawalki));
      odpowiedz(res, kod, tresc);
    }));
  } else if (req.method === 'GET') {
    if (sciezka !== '/' && sciezka !== '/raport') {
      return odpowiedz(res, 404, '{"error":"not found"}');
    }
    if (!zalogowany(req, res)) return;
    // zly query string = biezacy miesiac, nigdy 400
    let miesiac = new URLSearchParams(reszta.join('?')).get('miesiac') || '';
    if (!/^\d{4}-(0[1-9]|1[0-2])$/.test(miesiac)) miesiac = lokalny(Date.now() / 1000).slice(0, 7);
    odpowiedz(res, 200, raport(miesiac), 'text/html');
  } else {
    odpowiedz(res, 405, '{"error":"method"}');
  }
}

if (!HASLO) {
  console.error('Ustaw haslo zmienna RCP_HASLO (to samo co w czytniku, pole Haslo).');
  process.exit(1);
}
wczytajOdbicia();
console.log(`Serwer odbic na porcie ${PORT}, odbic w ${PLIK_ODBIC}: ${odbicia.length}`);
http.createServer((req, res) => bezpiecznie(res, () => obsluz(req, res))).listen(PORT);
