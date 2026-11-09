<?php
// Serwer odbic RAZ-RCP-1 - przyklad w PHP (bez zaleznosci, PHP 8.1+).
//
// Przyjmuje odbicia wysylane przez czytnik (Ustawienia > Integracje > Synchronizacja)
// i liczy z nich prosty czas pracy. Protokol i konfiguracja: ../README.md.
//
// Uruchomienie (wbudowany serwer PHP, ten plik jako router):
//   RCP_HASLO=tajne php -S 0.0.0.0:8080 index.php
//
// W czytniku: adres http://<ip-komputera>:8080/odbicia, format http3, login rcp, haslo tajne.
// Raport czasu pracy: GET na ten sam adres, np. http://<ip-komputera>:8080/raport.
//
// Pod Apache / nginx + PHP-FPM: adres w czytniku wskazuje ten plik
// (np. https://serwer.firma.pl/rcp/index.php), zmienne RCP_* ustaw w konfiguracji serwera.
// RCP_DANE (i RCP_PRACOWNICY) sa tam wymagane - sciezka poza katalogiem serwowanym przez WWW.

declare(strict_types=1);

const CZAS_MIN = 1767225600;     // 2026-01-01 UTC: wczesniej = zegar czytnika nieustawiony
const CIALO_MAX = 4096;          // czytnik wysyla kilkadziesiat bajtow
const MACHINE_MAX = 64;

function env(string $nazwa, string $domyslna): string
{
    $v = getenv($nazwa);
    return $v === false || $v === '' ? $domyslna : $v;
}

// Nieoczekiwany wyjatek -> 500 (kazde zadanie to osobny proces, serwer dziala dalej).
set_exception_handler(function (Throwable $e): void {
    error_log((string)$e);
    if (!headers_sent()) {
        odpowiedz(500, '{"error":"internal"}');
    }
});
// Zle bajty UTF-8 (od czytnika albo w plikach) -> U+FFFD, jak w pozostalych przykladach.
mb_substitute_character(0xFFFD);

// Strefa ze zmiennej srodowiska - zla nazwa: 500 i komunikat w logu przy kazdym zadaniu
// (wbudowany serwer PHP uruchamia ten plik dopiero przy zadaniu, nie przy starcie).
function strefa(string $zmienna, string $domyslna): DateTimeZone
{
    $nazwa = env($zmienna, $domyslna);
    try {
        return new DateTimeZone($nazwa);
    } catch (Exception) {
        error_log("Nieznana strefa czasowa $zmienna=\"$nazwa\" (np. UTC, Europe/Warsaw).");
        odpowiedz(500, "{\"error\":\"$zmienna\"}");
    }
}

$LOGIN = env('RCP_LOGIN', 'rcp');
$HASLO = env('RCP_HASLO', '');
// Pliki domyslnie obok skryptu tylko pod wbudowanym serwerem (ten router odpowiada na kazda
// sciezke). Pod Apache / nginx katalog skryptu jest serwowany - pliki dalyby sie pobrac bez hasla.
$WBUDOWANY = PHP_SAPI === 'cli-server';
$PLIK_ODBIC = env('RCP_DANE', $WBUDOWANY ? __DIR__ . '/odbicia.tsv' : '');
$PLIK_PRACOWNIKOW = env('RCP_PRACOWNICY', $WBUDOWANY ? __DIR__ . '/pracownicy.csv' : '');
// Strefa pola timestamp: UTC dla formatow http2/http3, Europe/Warsaw dla formatow Skryba.
$STREFA_CZYTNIKA = strefa('RCP_STREFA_CZYTNIKA', 'UTC');
// Strefa, w ktorej raport pokazuje godziny i dzieli odbicia na miesiace.
$STREFA = strefa('RCP_STREFA', 'Europe/Warsaw');

function odpowiedz(int $kod, string $tresc, string $typ = 'application/json'): never
{
    http_response_code($kod);
    header("Content-Type: $typ; charset=utf-8");
    echo $tresc;
    exit;
}

// Odbicia z otwartego pliku: linie "epoch<TAB>rfid<TAB>machineId" (LF albo CRLF)
// -> [klucz => [epoch, rfid, machine]]. Zle bajty -> U+FFFD zamiast bledu.
function wczytaj_odbicia($f): array
{
    $odbicia = [];
    rewind($f);
    while (($linia = fgets($f)) !== false) {
        $linia = mb_scrub(rtrim($linia, "\n"), 'UTF-8');
        if (str_ends_with($linia, "\r")) {
            $linia = substr($linia, 0, -1);
        }
        $pola = explode("\t", $linia);
        if (count($pola) !== 3 || !ctype_digit($pola[0])) {
            continue;
        }
        $odbicia[implode("\t", $pola)] = [(int)$pola[0], $pola[1], $pola[2]];
    }
    return $odbicia;
}

// "RRRR-MM-DD GG:MM:SS" w strefie czytnika -> epoch, null przy zlym zapisie.
function parsuj_czas(string $s): ?int
{
    global $STREFA_CZYTNIKA;
    if (!preg_match('/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/', $s)) {
        return null;
    }
    $dt = DateTimeImmutable::createFromFormat('!Y-m-d H:i:s', $s, new DateTimeZone('UTC'));
    $bledy = DateTimeImmutable::getLastErrors();       // np. 2026-02-30 to ostrzezenie, nie 2 marca
    if ($dt === false || ($bledy && ($bledy['warning_count'] || $bledy['error_count']))) {
        return null;
    }
    // Zmiana czasu: godzina podwojna jesienia -> pozniejsza (czas zimowy),
    // nieistniejaca wiosna -> z przesunieciem sprzed zmiany (02:30 = 03:30 CEST).
    // Liczone jawnie z przesuniec strefy, nie zalezy od wyboru DateTime.
    $zegar = $dt->getTimestamp();                      // czas czytnika, jak gdyby w UTC
    $przes = fn(int $e): int => $STREFA_CZYTNIKA->getOffset(new DateTimeImmutable("@$e"));
    $przed = $przes($zegar - 86400);
    $wynik = [];
    foreach ([$przed, $przes($zegar + 86400)] as $o) {
        if ($przes($zegar - $o) === $o) {
            $wynik[] = $zegar - $o;
        }
    }
    return $wynik ? max($wynik) : $zegar - $przed;
}

// Pierwsza wartosc pola z ciala formularza (query string w adresie jest pomijany):
// '+' = spacja, bajty z %XX jako UTF-8, zle sekwencje -> U+FFFD.
function pole(string $cialo, string $nazwa): string
{
    foreach (explode('&', $cialo) as $para) {
        [$klucz, $wartosc] = explode('=', $para, 2) + [1 => ''];
        if (urldecode($klucz) === $nazwa) {
            return mb_scrub(urldecode($wartosc), 'UTF-8');
        }
    }
    return '';
}

// Cialo formularza od czytnika -> [kod HTTP, odpowiedz JSON].
function przyjmij(string $cialo): array
{
    global $PLIK_ODBIC;
    $rfid = strtolower(trim(pole($cialo, 'rfidID')));
    $czas = trim(pole($cialo, 'timestamp'));
    $machine = str_replace(["\t", "\r", "\n"], ' ', pole($cialo, 'machineId'));
    $machine = mb_substr($machine, 0, MACHINE_MAX, 'UTF-8');

    // 400 = czytnik zdejmuje odbicie z kolejki i nie wysle go ponownie,
    // wiec tylko dla zadan, ktorych i tak nie da sie zapisac.
    if (!preg_match('/^[0-9a-f]{1,20}$/', $rfid)) {
        return [400, '{"stored":false,"error":"rfidID"}'];
    }
    $epoch = parsuj_czas($czas);
    if ($epoch === null) {
        return [400, '{"stored":false,"error":"timestamp"}'];
    }
    // Odbicie z nieustawionym zegarem: 200 z "stored":false i slowem "clock" -
    // czytnik zapisze w swoim logu, ze serwer go nie przyjal z powodu zegara.
    if ($epoch < CZAS_MIN) {
        return [200, '{"stored":false,"reason":"clock"}'];
    }

    $linia = "$epoch\t$rfid\t$machine";
    $f = @fopen($PLIK_ODBIC, 'c+');
    if ($f === false || !flock($f, LOCK_EX)) {     // blokada: kazde zadanie to osobny proces
        error_log("blad zapisu $PLIK_ODBIC");
        return [500, '{"stored":false,"error":"storage"}'];    // czytnik ponowi
    }
    try {
        // To samo odbicie drugi raz (np. odpowiedz zginela po drodze) - 200,
        // zeby czytnik przestal je wysylac.
        if (isset(wczytaj_odbicia($f)[$linia])) {
            return [200, '{"stored":false,"reason":"duplicate"}'];
        }
        // Ostatnia linia bez '\n' (awaria w trakcie zapisu) - konczymy ja, zeby
        // nowe odbicie nie dokleilo sie do niej.
        $koniec = fseek($f, -1, SEEK_END) === 0 && fread($f, 1) !== "\n" ? "\n" : '';
        // Na dysku przed odpowiedzia 200 (czytnik oznacza odbicie jako wyslane, a moze je usunac).
        fseek($f, 0, SEEK_END);
        if (fwrite($f, "$koniec$linia\n") === false || !fflush($f) || !fsync($f)) {
            error_log("blad zapisu $PLIK_ODBIC");
            return [500, '{"stored":false,"error":"storage"}'];
        }
    } finally {
        flock($f, LOCK_UN);
        fclose($f);
    }
    error_log(sprintf('odbicie %s %s z "%s"', $rfid, lokalny($epoch, 'Y-m-d H:i:s'), $machine));
    return [200, '{"stored":true}'];
}

// -- raport czasu pracy -------------------------------------------------------

// rfidID -> osoba z pliku "rfidID;Imie Nazwisko". Kilka kart z ta sama nazwa = jedna osoba.
// BOM na poczatku pliku jest zdejmowany, zle bajty (np. plik w Windows-1250) -> U+FFFD.
function wczytaj_pracownikow(): array
{
    global $PLIK_PRACOWNIKOW;
    $mapa = [];
    $linie = $PLIK_PRACOWNIKOW === '' ? [] : (@file($PLIK_PRACOWNIKOW, FILE_IGNORE_NEW_LINES) ?: []);
    foreach ($linie as $i => $linia) {
        if ($i === 0 && str_starts_with($linia, "\u{FEFF}")) {
            $linia = substr($linia, 3);
        }
        $linia = trim(mb_scrub($linia, 'UTF-8'));
        if ($linia === '' || $linia[0] === '#' || !str_contains($linia, ';')) {
            continue;
        }
        [$rfid, $osoba] = explode(';', $linia, 2);
        if (trim($osoba) !== '') {
            $mapa[strtolower(trim($rfid))] = trim($osoba);
        }
    }
    return $mapa;
}

// Odbicia -> [osoba => [[wejscie, wyjscie albo null]]] jak w panelu czytnika:
// kolejne odbicia osoby parowane 1.+2., 3.+4., ... bez ciecia na polnocy
// (zmiana 22:00-06:00 to jedna zmiana). Nieparzyste ostatnie = osoba w pracy.
function zmiany(array $odbicia, array $mapa): array
{
    $per = [];
    foreach ($odbicia as [$epoch, $rfid]) {
        $per[$mapa[$rfid] ?? "karta $rfid"][] = $epoch;
    }
    ksort($per, SORT_STRING);
    $wynik = [];
    foreach ($per as $osoba => $czasy) {
        sort($czasy);
        for ($i = 0; $i < count($czasy); $i += 2) {
            $wynik[$osoba][] = [$czasy[$i], $czasy[$i + 1] ?? null];
        }
    }
    return $wynik;
}

function lokalny(int $epoch, string $fmt = 'Y-m-d H:i'): string
{
    global $STREFA;
    return (new DateTimeImmutable("@$epoch"))->setTimezone($STREFA)->format($fmt);
}

function godziny(int $s): string
{
    return sprintf('%d:%02d', intdiv($s, 3600), intdiv($s % 3600, 60));
}

function raport(string $miesiac): string
{
    global $PLIK_ODBIC;
    $f = @fopen($PLIK_ODBIC, 'r');
    $odbicia = [];
    if ($f !== false) {
        flock($f, LOCK_SH);
        $odbicia = wczytaj_odbicia($f);
        fclose($f);
    }
    $w_pracy = $sumy = $wiersze = '';
    foreach (zmiany($odbicia, wczytaj_pracownikow()) as $osoba => $lista) {
        $e = htmlspecialchars((string)$osoba, ENT_QUOTES | ENT_SUBSTITUTE);
        $suma = 0;
        foreach ($lista as [$wej, $wyj]) {
            if ($wyj === null) {
                $w_pracy .= "<li>$e od " . lokalny($wej) . '</li>';
            }
            if (lokalny($wej, 'Y-m') !== $miesiac) {
                continue;
            }
            if ($wyj === null) {
                $wiersze .= "<tr><td>$e</td><td>" . lokalny($wej) . '</td><td>-</td><td>w pracy</td></tr>';
            } else {
                $suma += $wyj - $wej;
                $wiersze .= "<tr><td>$e</td><td>" . lokalny($wej) . '</td><td>' . lokalny($wyj)
                    . '</td><td>' . godziny($wyj - $wej) . '</td></tr>';
            }
        }
        if ($suma > 0) {
            $sumy .= "<tr><td>$e</td><td>" . godziny($suma) . '</td></tr>';
        }
    }
    $m = new DateTimeImmutable("$miesiac-01");
    $poprz = $m->modify('-1 month')->format('Y-m');
    $nast = $m->modify('+1 month')->format('Y-m');
    $n = count($odbicia);
    $w_pracy = $w_pracy !== '' ? "<ul>$w_pracy</ul>" : '<p>Nikt.</p>';
    return <<<HTML
        <!doctype html>
        <html lang="pl"><head><meta charset="utf-8"><title>Czas pracy $miesiac</title>
        <style>body{font-family:'IBM Plex Sans',system-ui,sans-serif;margin:2em;color:#16151D;background:#F8F8FA}
        h1,h2{font-family:Sora,system-ui,sans-serif;font-weight:600}a{color:#6A4CF6}
        table{border-collapse:collapse;background:#fff}td,th{border:1px solid #E4E4EC;padding:4px 8px;text-align:left}
        th{background:#EDE8FF}</style></head><body>
        <h1>Czas pracy $miesiac</h1>
        <p><a href="?miesiac=$poprz">&larr; $poprz</a> | <a href="?miesiac=$nast">$nast &rarr;</a>
        | odbic w bazie: $n</p>
        <h2>Teraz w pracy</h2>
        $w_pracy
        <h2>Suma w miesiacu</h2>
        <table><tr><th>Osoba</th><th>Czas pracy</th></tr>$sumy</table>
        <h2>Zmiany</h2>
        <table><tr><th>Osoba</th><th>Wejscie</th><th>Wyjscie</th><th>Czas</th></tr>$wiersze</table>
        </body></html>

        HTML;
}

// -- HTTP ---------------------------------------------------------------------

// HTTP Basic z RCP_LOGIN / RCP_HASLO. Przy bledzie 401 - czytnik przerywa wtedy cykl
// i ponawia pozniej. Pod CGI/FPM naglowek bywa tylko w HTTP_AUTHORIZATION.
function zalogowany(): bool
{
    global $LOGIN, $HASLO;
    $login = $_SERVER['PHP_AUTH_USER'] ?? null;
    $haslo = $_SERVER['PHP_AUTH_PW'] ?? '';
    $nagl = $_SERVER['HTTP_AUTHORIZATION'] ?? $_SERVER['REDIRECT_HTTP_AUTHORIZATION'] ?? '';
    if ($login === null && str_starts_with($nagl, 'Basic ')) {
        [$login, $haslo] = explode(':', (string)base64_decode(substr($nagl, 6), true), 2) + [1 => ''];
    }
    // hash_equals: czas porownania nie zdradza, ile znakow sie zgadza
    return $login !== null && hash_equals($LOGIN, $login) & hash_equals($HASLO, $haslo);
}

if ($HASLO === '') {
    error_log('Ustaw haslo zmienna RCP_HASLO (to samo co w czytniku, pole Haslo).');
    odpowiedz(500, '{"error":"RCP_HASLO"}');
}
if ($PLIK_ODBIC === '') {
    error_log('Ustaw RCP_DANE (i RCP_PRACOWNICY) na sciezke poza katalogiem serwowanym przez WWW.');
    odpowiedz(500, '{"error":"RCP_DANE"}');
}

$metoda = $_SERVER['REQUEST_METHOD'] ?? 'GET';
if ($metoda !== 'POST' && $metoda !== 'GET') {
    odpowiedz(405, '{"error":"method"}');
}
if ($metoda === 'POST') {
    $cialo = (string)file_get_contents('php://input', false, null, 0, CIALO_MAX + 1);
    if (strlen($cialo) > CIALO_MAX) {
        odpowiedz(413, '{"error":"body"}');
    }
}
if (!zalogowany()) {
    header('WWW-Authenticate: Basic realm="RCP"');
    odpowiedz(401, '{"error":"unauthorized"}');
}
if ($metoda === 'POST') {
    [$kod, $tresc] = przyjmij($cialo);
    odpowiedz($kod, $tresc);
}
$miesiac = is_string($_GET['miesiac'] ?? null) ? $_GET['miesiac'] : '';
if (!preg_match('/^\d{4}-(0[1-9]|1[0-2])$/', $miesiac)) {
    $miesiac = lokalny(time(), 'Y-m');
}
odpowiedz(200, raport($miesiac), 'text/html');
