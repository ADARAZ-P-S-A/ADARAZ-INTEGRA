// Serwer odbic RAZ-RCP-1 - przyklad w Javie (tylko JDK, Java 17+).
//
// Przyjmuje odbicia wysylane przez czytnik (Ustawienia > Integracje > Synchronizacja)
// i liczy z nich prosty czas pracy. Protokol i konfiguracja: ../README.md.
//
// Uruchomienie (bez kompilacji, jeden plik):
//   RCP_HASLO=tajne java RcpServer.java
//
// W czytniku: adres http://<ip-komputera>:8080/odbicia, format http3, login rcp, haslo tajne.
// Raport czasu pracy: http://<ip-komputera>:8080/raport (ten sam login i haslo).

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.InetSocketAddress;
import java.nio.ByteBuffer;
import java.nio.channels.FileChannel;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.security.MessageDigest;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.DateTimeException;
import java.time.YearMonth;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;
import java.time.format.ResolverStyle;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.concurrent.Executors;
import java.util.regex.Pattern;

public class RcpServer {

    static final long CZAS_MIN = 1767225600L;   // 2026-01-01 UTC: wczesniej = zegar czytnika nieustawiony
    static final int CIALO_MAX = 4096;          // czytnik wysyla kilkadziesiat bajtow
    static final int MACHINE_MAX = 64;
    static final Pattern RFID = Pattern.compile("[0-9a-f]{1,20}");
    static final Pattern MIESIAC = Pattern.compile("\\d{4}-(0[1-9]|1[0-2])");
    static final Pattern CZAS = Pattern.compile("\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}");
    // uuuu + STRICT: 2026-02-30 to blad, nie 28 lutego
    static final DateTimeFormatter CZAS_CZYTNIKA =
            DateTimeFormatter.ofPattern("uuuu-MM-dd HH:mm:ss").withResolverStyle(ResolverStyle.STRICT);
    static final DateTimeFormatter DZIEN = DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm");
    static final DateTimeFormatter SEKUNDY = DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss");

    static final String LOGIN = env("RCP_LOGIN", "rcp");
    static final String HASLO = env("RCP_HASLO", "");
    static final Path PLIK_ODBIC = Path.of(env("RCP_DANE", "odbicia.tsv"));
    static final Path PLIK_PRACOWNIKOW = Path.of(env("RCP_PRACOWNICY", "pracownicy.csv"));
    // Strefa pola timestamp: UTC dla formatow http2/http3, Europe/Warsaw dla formatow Skryba.
    static final ZoneId STREFA_CZYTNIKA = strefa("RCP_STREFA_CZYTNIKA", "UTC");
    // Strefa, w ktorej raport pokazuje godziny i dzieli odbicia na miesiace.
    static final ZoneId STREFA = strefa("RCP_STREFA", "Europe/Warsaw");

    record Odbicie(long epoch, String rfid, String machine) {}

    record Zmiana(long wej, Long wyj) {}         // wyj == null: brak wyjscia, osoba w pracy

    static final Object blokada = new Object();
    static final List<Odbicie> odbicia = new ArrayList<>();   // w kolejnosci przyjecia
    static final Set<Odbicie> znane = new HashSet<>();        // do wykrywania duplikatow

    static String env(String nazwa, String domyslna) {
        String v = System.getenv(nazwa);
        return v == null || v.isEmpty() ? domyslna : v;
    }

    // Strefa ze zmiennej srodowiska - zla nazwa konczy program od razu przy starcie.
    static ZoneId strefa(String zmienna, String domyslna) {
        String nazwa = env(zmienna, domyslna);
        try {
            return ZoneId.of(nazwa);
        } catch (DateTimeException e) {
            System.err.printf("Nieznana strefa czasowa %s=\"%s\" (np. UTC, Europe/Warsaw).%n", zmienna, nazwa);
            System.exit(1);
            return null;
        }
    }

    // Bajty -> tekst UTF-8, kazda zla sekwencja -> U+FFFD (jak WHATWG i Python errors="replace";
    // new String(..., UTF_8) liczy zle sekwencje inaczej).
    static String utf8(byte[] b) {
        StringBuilder s = new StringBuilder(b.length);
        int i = 0;
        while (i < b.length) {
            int c = b[i] & 0xff, n, lo = 0x80, hi = 0xbf;
            if (c < 0x80) {
                s.append((char) c);
                i++;
                continue;
            }
            if (c >= 0xc2 && c <= 0xdf) n = 1;
            else if (c >= 0xe0 && c <= 0xef) { n = 2; if (c == 0xe0) lo = 0xa0; if (c == 0xed) hi = 0x9f; }
            else if (c >= 0xf0 && c <= 0xf4) { n = 3; if (c == 0xf0) lo = 0x90; if (c == 0xf4) hi = 0x8f; }
            else {
                s.append('\uFFFD');
                i++;
                continue;
            }
            int cp = c & (0x3f >> n), j = i + 1;
            for (int k = 0; k < n && j < b.length; k++, j++) {
                int d = b[j] & 0xff;
                if (d < lo || d > hi) break;
                cp = cp << 6 | d & 0x3f;
                lo = 0x80;
                hi = 0xbf;
            }
            if (j - i == n + 1) s.appendCodePoint(cp);
            else s.append('\uFFFD');   // ucieta sekwencja = jeden U+FFFD, zly bajt czytany od nowa
            i = j;
        }
        return s.toString();
    }

    // Plik jako UTF-8 (zle bajty -> U+FFFD) podzielony na linie po '\n', bez '\r' na koncu (CRLF).
    static List<String> linie(Path plik) throws IOException {
        List<String> wynik = new ArrayList<>();
        for (String linia : utf8(Files.readAllBytes(plik)).split("\n", -1)) {
            wynik.add(linia.endsWith("\r") ? linia.substring(0, linia.length() - 1) : linia);
        }
        return wynik;
    }

    // Odbicia z pliku: linie "epoch<TAB>rfid<TAB>machineId" (LF albo CRLF).
    static void wczytajOdbicia() throws IOException {
        List<String> linie;
        try {
            // Ostatnia linia bez '\n' (awaria w trakcie zapisu) - konczymy ja, zeby
            // nastepne odbicie nie dokleilo sie do niej.
            try (FileChannel f = FileChannel.open(PLIK_ODBIC, StandardOpenOption.READ, StandardOpenOption.WRITE)) {
                ByteBuffer b = ByteBuffer.allocate(1);
                if (f.size() > 0 && f.read(b, f.size() - 1) == 1 && b.get(0) != '\n') {
                    f.write(ByteBuffer.wrap(new byte[] {'\n'}), f.size());
                    f.force(true);
                }
            }
            linie = linie(PLIK_ODBIC);
        } catch (NoSuchFileException e) {
            return;
        }
        for (String linia : linie) {
            String[] pola = linia.split("\t", -1);
            if (pola.length != 3 || !pola[0].matches("\\d+")) continue;
            long epoch;
            try {
                epoch = Long.parseLong(pola[0]);
            } catch (NumberFormatException e) {
                continue;   // liczba za duza na long
            }
            Odbicie o = new Odbicie(epoch, pola[1], pola[2]);
            if (znane.add(o)) odbicia.add(o);
        }
    }

    // Dopisuje odbicie do pliku i czeka, az bedzie na dysku - dopiero wtedy wolno
    // odpowiedziec 200 (czytnik oznacza odbicie jako wyslane, a moze je usunac).
    static void zapiszOdbicie(Odbicie o) throws IOException {
        try (FileChannel f = FileChannel.open(PLIK_ODBIC, StandardOpenOption.CREATE,
                StandardOpenOption.WRITE, StandardOpenOption.APPEND)) {
            String linia = o.epoch() + "\t" + o.rfid() + "\t" + o.machine() + "\n";
            ByteBuffer b = ByteBuffer.wrap(linia.getBytes(StandardCharsets.UTF_8));
            while (b.hasRemaining()) f.write(b);
            f.force(true);
        }
    }

    // "RRRR-MM-DD GG:MM:SS" w strefie czytnika -> epoch, null przy zlym zapisie.
    static Long parsujCzas(String s) {
        // samo uuuu przyjeloby tez "+20260-..." i rok 0000
        if (!CZAS.matcher(s).matches() || s.startsWith("0000")) return null;
        try {
            // Zmiana czasu: godzina podwojna jesienia -> pozniejsza (czas zimowy),
            // nieistniejaca wiosna -> z przesunieciem sprzed zmiany (02:30 = 03:30 CEST).
            return LocalDateTime.parse(s, CZAS_CZYTNIKA).atZone(STREFA_CZYTNIKA)
                    .withLaterOffsetAtOverlap().toEpochSecond();
        } catch (DateTimeParseException e) {
            return null;
        }
    }

    // %XX -> bajt, '+' -> spacja; zle %.. zostaje doslownie (jak Python unquote).
    static String dekoduj(byte[] b, int od, int dok) {
        ByteArrayOutputStream w = new ByteArrayOutputStream();
        for (int i = od; i < dok; i++) {
            int c = b[i];
            if (c == '+') c = ' ';
            else if (c == '%' && i + 2 < dok && Character.digit(b[i + 1], 16) >= 0 && Character.digit(b[i + 2], 16) >= 0) {
                c = Character.digit(b[i + 1], 16) * 16 + Character.digit(b[i + 2], 16);
                i += 2;
            }
            w.write(c);
        }
        return utf8(w.toByteArray());   // bajty jako UTF-8, zle sekwencje -> U+FFFD
    }

    // application/x-www-form-urlencoded jak Python parse_qs: pary po '&', pierwsza
    // wartosc wygrywa, "nazwa" bez '=' to pusta wartosc (pomijana, gdy !puste).
    static Map<String, String> formularz(byte[] cialo, boolean puste) {
        Map<String, String> pola = new HashMap<>();
        int od = 0;
        while (od <= cialo.length) {
            int dok = od, rowna = -1;
            while (dok < cialo.length && cialo[dok] != '&') {
                if (cialo[dok] == '=' && rowna < 0) rowna = dok;
                dok++;
            }
            if (dok > od && (puste || rowna >= 0 && rowna + 1 < dok)) {
                pola.putIfAbsent(dekoduj(cialo, od, rowna < 0 ? dok : rowna),
                        rowna < 0 ? "" : dekoduj(cialo, rowna + 1, dok));
            }
            od = dok + 1;
        }
        return pola;
    }

    // Cialo formularza (bajty) od czytnika -> {kod HTTP, odpowiedz JSON}.
    static Object[] przyjmij(byte[] cialo) {
        Map<String, String> pola = formularz(cialo, true);
        String rfid = pola.getOrDefault("rfidID", "").trim().toLowerCase();
        String machine = pola.getOrDefault("machineId", "").replaceAll("[\t\r\n]", " ");
        if (machine.codePointCount(0, machine.length()) > MACHINE_MAX) {
            machine = machine.substring(0, machine.offsetByCodePoints(0, MACHINE_MAX));
        }

        // 400 = czytnik zdejmuje odbicie z kolejki i nie wysle go ponownie,
        // wiec tylko dla zadan, ktorych i tak nie da sie zapisac.
        if (!RFID.matcher(rfid).matches()) return new Object[] {400, "{\"stored\":false,\"error\":\"rfidID\"}"};
        Long epoch = parsujCzas(pola.getOrDefault("timestamp", "").trim());
        if (epoch == null) return new Object[] {400, "{\"stored\":false,\"error\":\"timestamp\"}"};
        // Odbicie z nieustawionym zegarem: 200 z "stored":false i slowem "clock" -
        // czytnik zapisze w swoim logu, ze serwer go nie przyjal z powodu zegara.
        if (epoch < CZAS_MIN) return new Object[] {200, "{\"stored\":false,\"reason\":\"clock\"}"};

        Odbicie o = new Odbicie(epoch, rfid, machine);
        synchronized (blokada) {
            // To samo odbicie drugi raz (np. odpowiedz zginela po drodze) - 200,
            // zeby czytnik przestal je wysylac.
            if (znane.contains(o)) return new Object[] {200, "{\"stored\":false,\"reason\":\"duplicate\"}"};
            try {
                zapiszOdbicie(o);
            } catch (IOException e) {
                System.err.println("blad zapisu " + PLIK_ODBIC + ": " + e);
                return new Object[] {500, "{\"stored\":false,\"error\":\"storage\"}"};   // czytnik ponowi
            }
            znane.add(o);
            odbicia.add(o);
        }
        System.out.printf("odbicie %s %s z \"%s\"%n", rfid, lokalny(epoch, SEKUNDY), machine);
        return new Object[] {200, "{\"stored\":true}"};
    }

    // -- raport czasu pracy ---------------------------------------------------

    // rfidID -> osoba z pliku "rfidID;Imie Nazwisko" (czytany przy kazdym raporcie).
    // Kilka kart z ta sama nazwa = jedna osoba. Zle bajty (np. plik w Windows-1250)
    // -> U+FFFD zamiast bledu, BOM na poczatku zdejmowany.
    static Map<String, String> wczytajPracownikow() {
        Map<String, String> mapa = new HashMap<>();
        List<String> linie;
        try {
            linie = linie(PLIK_PRACOWNIKOW);
        } catch (IOException e) {
            return mapa;
        }
        if (!linie.isEmpty() && linie.get(0).startsWith("\uFEFF")) linie.set(0, linie.get(0).substring(1));
        for (String linia : linie) {
            linia = linia.strip();
            int i = linia.indexOf(';');
            if (linia.isEmpty() || linia.startsWith("#") || i < 0) continue;
            String osoba = linia.substring(i + 1).strip();
            if (!osoba.isEmpty()) mapa.put(linia.substring(0, i).strip().toLowerCase(), osoba);
        }
        return mapa;
    }

    // Odbicia -> zmiany per osoba jak w panelu czytnika: kolejne odbicia osoby
    // parowane 1.+2., 3.+4., ... bez ciecia na polnocy (zmiana 22:00-06:00 to jedna
    // zmiana). Nieparzyste ostatnie = osoba w pracy. TreeMap - osoby alfabetycznie.
    static TreeMap<String, List<Zmiana>> zmiany(List<Odbicie> lista, Map<String, String> mapa) {
        Map<String, List<Long>> per = new HashMap<>();
        for (Odbicie o : lista) {
            String osoba = mapa.getOrDefault(o.rfid(), "karta " + o.rfid());
            per.computeIfAbsent(osoba, k -> new ArrayList<>()).add(o.epoch());
        }
        TreeMap<String, List<Zmiana>> wynik = new TreeMap<>();
        per.forEach((osoba, czasy) -> {
            czasy.sort(null);
            List<Zmiana> z = new ArrayList<>();
            for (int i = 0; i < czasy.size(); i += 2) {
                z.add(new Zmiana(czasy.get(i), i + 1 < czasy.size() ? czasy.get(i + 1) : null));
            }
            wynik.put(osoba, z);
        });
        return wynik;
    }

    static String lokalny(long epoch, DateTimeFormatter f) {
        return f.format(Instant.ofEpochSecond(epoch).atZone(STREFA));
    }

    static String godziny(long s) {
        return String.format("%d:%02d", s / 3600, s % 3600 / 60);
    }

    static String esc(String s) {
        return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
                .replace("\"", "&quot;").replace("'", "&#39;");
    }

    static String raport(String miesiac) {
        List<Odbicie> kopia;
        synchronized (blokada) {
            kopia = new ArrayList<>(odbicia);
        }
        StringBuilder wPracy = new StringBuilder(), sumy = new StringBuilder(), wiersze = new StringBuilder();
        zmiany(kopia, wczytajPracownikow()).forEach((osoba, lista) -> {
            String e = esc(osoba);
            long suma = 0;
            for (Zmiana z : lista) {
                if (z.wyj() == null) wPracy.append("<li>" + e + " od " + lokalny(z.wej(), DZIEN) + "</li>");
                if (!lokalny(z.wej(), DZIEN).startsWith(miesiac)) continue;
                if (z.wyj() == null) {
                    wiersze.append("<tr><td>" + e + "</td><td>" + lokalny(z.wej(), DZIEN)
                            + "</td><td>-</td><td>w pracy</td></tr>");
                } else {
                    suma += z.wyj() - z.wej();
                    wiersze.append("<tr><td>" + e + "</td><td>" + lokalny(z.wej(), DZIEN) + "</td><td>"
                            + lokalny(z.wyj(), DZIEN) + "</td><td>" + godziny(z.wyj() - z.wej()) + "</td></tr>");
                }
            }
            if (suma > 0) sumy.append("<tr><td>" + e + "</td><td>" + godziny(suma) + "</td></tr>");
        });
        YearMonth ym = YearMonth.parse(miesiac);
        return SZABLON
                .replace("{m}", miesiac)
                .replace("{poprz}", ym.minusMonths(1).toString())
                .replace("{nast}", ym.plusMonths(1).toString())
                .replace("{n}", String.valueOf(kopia.size()))
                .replace("{w_pracy}", wPracy.length() > 0 ? "<ul>" + wPracy + "</ul>" : "<p>Nikt.</p>")
                .replace("{sumy}", sumy)
                .replace("{zmiany}", wiersze);
    }

    static final String SZABLON = """
            <!doctype html>
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
            """;

    // -- HTTP -----------------------------------------------------------------

    static void odpowiedz(HttpExchange x, int kod, String tresc, String typ) throws IOException {
        byte[] dane = tresc.getBytes(StandardCharsets.UTF_8);
        x.getResponseHeaders().set("Content-Type", typ + "; charset=utf-8");
        x.sendResponseHeaders(kod, dane.length);
        x.getResponseBody().write(dane);
        x.close();
    }

    // MessageDigest.isEqual na skrotach: czas porownania nie zdradza, ile znakow sie zgadza.
    static boolean rowne(String a, String b) throws Exception {
        MessageDigest sha = MessageDigest.getInstance("SHA-256");
        byte[] ha = sha.digest(a.getBytes(StandardCharsets.UTF_8));
        byte[] hb = sha.digest(b.getBytes(StandardCharsets.UTF_8));
        return MessageDigest.isEqual(ha, hb);
    }

    // HTTP Basic z RCP_LOGIN / RCP_HASLO. Przy bledzie odpowiada 401 sam - czytnik
    // przerywa wtedy cykl i ponawia pozniej.
    static boolean zalogowany(HttpExchange x) throws Exception {
        String nagl = x.getRequestHeaders().getFirst("Authorization");
        if (nagl != null && nagl.startsWith("Basic ")) {
            try {
                String cred = new String(Base64.getDecoder().decode(nagl.substring(6).trim()), StandardCharsets.UTF_8);
                int i = cred.indexOf(':');
                String login = i >= 0 ? cred.substring(0, i) : cred;
                String haslo = i >= 0 ? cred.substring(i + 1) : "";
                if (i >= 0 & rowne(login, LOGIN) & rowne(haslo, HASLO)) return true;
            } catch (IllegalArgumentException e) {
                // zly base64 - jak zle haslo
            }
        }
        x.getResponseHeaders().set("WWW-Authenticate", "Basic realm=\"RCP\"");
        odpowiedz(x, 401, "{\"error\":\"unauthorized\"}", "application/json");
        return false;
    }

    static void obsluz(HttpExchange x) throws IOException {
        try {
            String metoda = x.getRequestMethod();
            if (metoda.equals("POST")) {
                // Cialo czytane przed sprawdzeniem hasla - inaczej zostaloby w polaczeniu keep-alive.
                InputStream in = x.getRequestBody();
                byte[] cialo = in.readNBytes(CIALO_MAX + 1);
                if (cialo.length > CIALO_MAX) {
                    odpowiedz(x, 413, "{\"error\":\"body\"}", "application/json");
                    return;
                }
                if (!zalogowany(x)) return;
                // Formularz tylko z ciala - query string w adresie jest pomijany.
                Object[] w = przyjmij(cialo);
                odpowiedz(x, (Integer) w[0], (String) w[1], "application/json");
            } else if (metoda.equals("GET")) {
                String sciezka = x.getRequestURI().getRawPath();
                if (!sciezka.equals("/") && !sciezka.equals("/raport")) {
                    odpowiedz(x, 404, "{\"error\":\"not found\"}", "application/json");
                    return;
                }
                if (!zalogowany(x)) return;
                // zly query string = biezacy miesiac, nigdy 400
                String query = x.getRequestURI().getRawQuery();
                String miesiac = formularz((query == null ? "" : query).getBytes(StandardCharsets.ISO_8859_1), false)
                        .getOrDefault("miesiac", "");
                if (!MIESIAC.matcher(miesiac).matches()) {
                    miesiac = YearMonth.now(STREFA).toString();
                }
                odpowiedz(x, 200, raport(miesiac), "text/html");
            } else {
                odpowiedz(x, 405, "{\"error\":\"method\"}", "application/json");
            }
        } catch (Exception e) {
            System.err.println("blad obslugi zadania: " + e);
            try {
                odpowiedz(x, 500, "{\"error\":\"internal\"}", "application/json");
            } catch (Exception e2) {
                x.close();   // odpowiedz juz zaczeta albo klient rozlaczony
            }
        }
    }

    public static void main(String[] args) throws IOException {
        if (HASLO.isEmpty()) {
            System.err.println("Ustaw haslo zmienna RCP_HASLO (to samo co w czytniku, pole Haslo).");
            System.exit(1);
        }
        // JVM dekoduje zmienne srodowiska wg locale - przy innym niz UTF-8 haslo z polskimi
        // znakami nie zgodzi sie z wyslanym przez czytnik.
        if (!"UTF-8".equalsIgnoreCase(System.getProperty("sun.jnu.encoding", ""))) {
            System.err.println("Uwaga: locale nie jest UTF-8 (sun.jnu.encoding=" + System.getProperty("sun.jnu.encoding")
                    + ") - haslo RCP_HASLO ze znakami spoza ASCII nie zgodzi sie z czytnikiem."
                    + " Uruchom z LANG=C.UTF-8.");
        }
        wczytajOdbicia();
        int port = Integer.parseInt(env("RCP_PORT", "8080"));
        // Zawieszony klient: po 30 s bez pelnego zadania / odpowiedzi polaczenie jest zamykane.
        if (System.getProperty("sun.net.httpserver.maxReqTime") == null) {
            System.setProperty("sun.net.httpserver.maxReqTime", "30");
        }
        if (System.getProperty("sun.net.httpserver.maxRspTime") == null) {
            System.setProperty("sun.net.httpserver.maxRspTime", "30");
        }
        HttpServer srv = HttpServer.create(new InetSocketAddress(port), 0);
        srv.createContext("/", RcpServer::obsluz);
        // watek na zadanie - wolni klienci nie blokuja pozostalych
        srv.setExecutor(Executors.newCachedThreadPool());
        srv.start();
        System.out.printf("Serwer odbic na porcie %d, odbic w %s: %d%n", port, PLIK_ODBIC, odbicia.size());
    }
}
