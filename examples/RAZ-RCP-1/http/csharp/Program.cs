// Serwer odbic RAZ-RCP-1 - przyklad w C# (ASP.NET Core minimal API, .NET 10).
//
// Przyjmuje odbicia wysylane przez czytnik (Ustawienia > Integracje > Synchronizacja)
// i liczy z nich prosty czas pracy. Protokol i konfiguracja: ../README.md.
//
// Uruchomienie:
//   RCP_HASLO=tajne dotnet run
//   (Windows PowerShell: $env:RCP_HASLO="tajne"; dotnet run)
//
// W czytniku: adres http://<ip-komputera>:8080/odbicia, format http3, login rcp, haslo tajne.
// Raport czasu pracy: http://<ip-komputera>:8080/raport (ten sam login i haslo).

using System.Globalization;
using System.Net;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;

const long CzasMin = 1767225600;   // 2026-01-01 UTC: wczesniej = zegar czytnika nieustawiony
const int CialoMax = 4096;         // czytnik wysyla kilkadziesiat bajtow
const int MachineMax = 64;

string Env(string nazwa, string domyslna) =>
    Environment.GetEnvironmentVariable(nazwa) is { Length: > 0 } v ? v : domyslna;

var login = Env("RCP_LOGIN", "rcp");
var haslo = Env("RCP_HASLO", "");
var plikOdbic = Env("RCP_DANE", "odbicia.tsv");
var plikPracownikow = Env("RCP_PRACOWNICY", "pracownicy.csv");

// Strefa ze zmiennej srodowiska - zla nazwa konczy program od razu przy starcie.
TimeZoneInfo Strefa(string zmienna, string domyslna)
{
    var nazwa = Env(zmienna, domyslna);
    try
    {
        return TimeZoneInfo.FindSystemTimeZoneById(nazwa);
    }
    catch (Exception e) when (e is TimeZoneNotFoundException or InvalidTimeZoneException)
    {
        Console.Error.WriteLine($"Nieznana strefa czasowa {zmienna}='{nazwa}' (np. UTC, Europe/Warsaw).");
        Environment.Exit(1);
        throw;
    }
}

// Strefa pola timestamp: UTC dla formatow http2/http3, Europe/Warsaw dla formatow Skryba.
var strefaCzytnika = Strefa("RCP_STREFA_CZYTNIKA", "UTC");
// Strefa, w ktorej raport pokazuje godziny i dzieli odbicia na miesiace.
var strefa = Strefa("RCP_STREFA", "Europe/Warsaw");
var port = Env("RCP_PORT", "8080");

if (haslo == "")
{
    Console.Error.WriteLine("Ustaw haslo zmienna RCP_HASLO (to samo co w czytniku, pole Haslo).");
    return 1;
}

var blokada = new object();
var odbicia = new List<Odbicie>();         // w kolejnosci przyjecia
var znane = new HashSet<Odbicie>();        // do wykrywania duplikatow

// Odbicia z pliku: linie "epoch<TAB>rfid<TAB>machineId" (LF albo CRLF).
if (File.Exists(plikOdbic))
{
    // Ostatnia linia bez '\n' (awaria w trakcie zapisu) - konczymy ja, zeby
    // nastepne odbicie nie dokleilo sie do niej.
    using (var f = new FileStream(plikOdbic, FileMode.Open, FileAccess.ReadWrite, FileShare.Read))
    {
        if (f.Length > 0 && f.Seek(-1, SeekOrigin.End) >= 0 && f.ReadByte() != '\n')
        {
            f.WriteByte((byte)'\n');
            f.Flush(flushToDisk: true);
        }
    }
    foreach (var linia in Tekst.Linie(File.ReadAllBytes(plikOdbic)))
    {
        var pola = linia.Split('\t');
        if (pola.Length != 3 || !long.TryParse(pola[0], NumberStyles.None, CultureInfo.InvariantCulture, out var e))
            continue;
        var o = new Odbicie(e, pola[1], pola[2]);
        if (znane.Add(o)) odbicia.Add(o);
    }
}

// Dopisuje odbicie do pliku i czeka, az bedzie na dysku - dopiero wtedy wolno
// odpowiedziec 200 (czytnik oznacza odbicie jako wyslane, a moze je usunac).
void ZapiszOdbicie(Odbicie o)
{
    using var f = new FileStream(plikOdbic, FileMode.Append, FileAccess.Write, FileShare.Read);
    var dane = Encoding.UTF8.GetBytes($"{o.Epoch}\t{o.Rfid}\t{o.Machine}\n");
    f.Write(dane);
    f.Flush(flushToDisk: true);
}

// "RRRR-MM-DD GG:MM:SS" w strefie czytnika -> epoch, null przy zlym zapisie.
long? ParsujCzas(string s)
{
    if (!DateTime.TryParseExact(s, "yyyy-MM-dd HH:mm:ss", CultureInfo.InvariantCulture,
            DateTimeStyles.None, out var dt))
        return null;
    // Godzina, ktorej nie ma (przestawienie zegara wiosna, formaty Skryba) - jak zegar po zmianie.
    if (strefaCzytnika.IsInvalidTime(dt)) dt = dt.AddHours(1);
    return new DateTimeOffset(TimeZoneInfo.ConvertTimeToUtc(dt, strefaCzytnika)).ToUnixTimeSeconds();
}

// Cialo formularza (bajty) od czytnika -> (kod HTTP, odpowiedz JSON).
(int, string) Przyjmij(ReadOnlySpan<byte> cialo)
{
    var pola = Tekst.Formularz(cialo);
    string Pole(string k) => pola.GetValueOrDefault(k, "");
    var rfid = Pole("rfidID").Trim().ToLowerInvariant();
    var machine = Regex.Replace(Pole("machineId"), "[\t\r\n]", " ");
    machine = Tekst.Pierwsze(machine, MachineMax);

    // 400 = czytnik zdejmuje odbicie z kolejki i nie wysle go ponownie,
    // wiec tylko dla zadan, ktorych i tak nie da sie zapisac.
    if (!Regex.IsMatch(rfid, "^[0-9a-f]{1,20}$")) return (400, "{\"stored\":false,\"error\":\"rfidID\"}");
    if (ParsujCzas(Pole("timestamp").Trim()) is not long epoch)
        return (400, "{\"stored\":false,\"error\":\"timestamp\"}");
    // Odbicie z nieustawionym zegarem: 200 z "stored":false i slowem "clock" -
    // czytnik zapisze w swoim logu, ze serwer go nie przyjal z powodu zegara.
    if (epoch < CzasMin) return (200, "{\"stored\":false,\"reason\":\"clock\"}");

    var o = new Odbicie(epoch, rfid, machine);
    lock (blokada)
    {
        // To samo odbicie drugi raz (np. odpowiedz zginela po drodze) - 200,
        // zeby czytnik przestal je wysylac.
        if (znane.Contains(o)) return (200, "{\"stored\":false,\"reason\":\"duplicate\"}");
        try
        {
            ZapiszOdbicie(o);
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException)
        {
            Console.Error.WriteLine($"blad zapisu {plikOdbic}: {e.Message}");
            return (500, "{\"stored\":false,\"error\":\"storage\"}");   // czytnik ponowi
        }
        znane.Add(o);
        odbicia.Add(o);
    }
    Console.WriteLine($"odbicie {rfid} {Lokalny(epoch, "yyyy-MM-dd HH:mm:ss")} z \"{machine}\"");
    return (200, "{\"stored\":true}");
}

// -- raport czasu pracy -------------------------------------------------------

// rfidID -> osoba z pliku "rfidID;Imie Nazwisko" (czytany przy kazdym raporcie).
// Kilka kart z ta sama nazwa = jedna osoba.
Dictionary<string, string> WczytajPracownikow()
{
    var mapa = new Dictionary<string, string>();
    if (!File.Exists(plikPracownikow)) return mapa;
    // BOM zdejmowany, zle bajty (np. plik w Windows-1250) -> U+FFFD zamiast bledu.
    foreach (var surowa in Tekst.Linie(File.ReadAllBytes(plikPracownikow), bom: true))
    {
        var linia = surowa.Trim();
        var i = linia.IndexOf(';');
        if (linia == "" || linia.StartsWith('#') || i < 0) continue;
        var osoba = linia[(i + 1)..].Trim();
        if (osoba != "") mapa[linia[..i].Trim().ToLowerInvariant()] = osoba;
    }
    return mapa;
}

// Odbicia -> zmiany per osoba jak w panelu czytnika: kolejne odbicia osoby
// parowane 1.+2., 3.+4., ... bez ciecia na polnocy (zmiana 22:00-06:00 to jedna
// zmiana). Nieparzyste ostatnie = osoba w pracy (Wyj == null).
SortedDictionary<string, List<(long Wej, long? Wyj)>> Zmiany(List<Odbicie> lista, Dictionary<string, string> mapa)
{
    var wynik = new SortedDictionary<string, List<(long, long?)>>(StringComparer.Ordinal);
    foreach (var g in lista.GroupBy(o => mapa.GetValueOrDefault(o.Rfid, $"karta {o.Rfid}")))
    {
        var czasy = g.Select(o => o.Epoch).Order().ToList();
        var z = new List<(long, long?)>();
        for (var i = 0; i < czasy.Count; i += 2)
            z.Add((czasy[i], i + 1 < czasy.Count ? czasy[i + 1] : null));
        wynik[g.Key] = z;
    }
    return wynik;
}

string Lokalny(long epoch, string fmt = "yyyy-MM-dd HH:mm") =>
    TimeZoneInfo.ConvertTime(DateTimeOffset.FromUnixTimeSeconds(epoch), strefa)
        .ToString(fmt, CultureInfo.InvariantCulture);

static string Godziny(long s) => $"{s / 3600}:{s % 3600 / 60:00}";

// "RRRR-MM" +/- miesiac bez DateTime - tez dla roku 0000 i 9999 (jak w Pythonie).
static string Sasiedni(string miesiac, int krok)
{
    int r = int.Parse(miesiac[..4], CultureInfo.InvariantCulture), m = int.Parse(miesiac[5..], CultureInfo.InvariantCulture) + krok;
    if (m < 1) (r, m) = (r - 1, 12);
    else if (m > 12) (r, m) = (r + 1, 1);
    return (r < 0 ? $"-{-r:000}" : $"{r:0000}") + $"-{m:00}";
}

string Raport(string miesiac)
{
    List<Odbicie> kopia;
    lock (blokada) kopia = new List<Odbicie>(odbicia);
    var wPracy = new StringBuilder();
    var sumy = new StringBuilder();
    var wiersze = new StringBuilder();
    foreach (var (osoba, lista) in Zmiany(kopia, WczytajPracownikow()))
    {
        var e = WebUtility.HtmlEncode(osoba);
        long suma = 0;
        foreach (var (wej, wyj) in lista)
        {
            if (wyj is null) wPracy.Append($"<li>{e} od {Lokalny(wej)}</li>");
            if (Lokalny(wej, "yyyy-MM") != miesiac) continue;
            if (wyj is long w)
            {
                suma += w - wej;
                wiersze.Append($"<tr><td>{e}</td><td>{Lokalny(wej)}</td><td>{Lokalny(w)}</td><td>{Godziny(w - wej)}</td></tr>");
            }
            else
            {
                wiersze.Append($"<tr><td>{e}</td><td>{Lokalny(wej)}</td><td>-</td><td>w pracy</td></tr>");
            }
        }
        if (suma > 0) sumy.Append($"<tr><td>{e}</td><td>{Godziny(suma)}</td></tr>");
    }
    var poprz = Sasiedni(miesiac, -1);
    var nast = Sasiedni(miesiac, 1);
    return Szablon.Html
        .Replace("{m}", miesiac).Replace("{poprz}", poprz).Replace("{nast}", nast)
        .Replace("{n}", kopia.Count.ToString(CultureInfo.InvariantCulture))
        .Replace("{w_pracy}", wPracy.Length > 0 ? $"<ul>{wPracy}</ul>" : "<p>Nikt.</p>")
        .Replace("{sumy}", sumy.ToString()).Replace("{zmiany}", wiersze.ToString());
}

// -- HTTP ---------------------------------------------------------------------

// FixedTimeEquals na skrotach: czas porownania nie zdradza, ile znakow sie zgadza.
static bool Rowne(string a, string b) =>
    CryptographicOperations.FixedTimeEquals(SHA256.HashData(Encoding.UTF8.GetBytes(a)),
                                            SHA256.HashData(Encoding.UTF8.GetBytes(b)));

// HTTP Basic z RCP_LOGIN / RCP_HASLO. Przy bledzie 401 - czytnik przerywa wtedy cykl
// i ponawia pozniej.
bool Zalogowany(HttpRequest req)
{
    var nagl = req.Headers.Authorization.ToString();
    if (!nagl.StartsWith("Basic ", StringComparison.Ordinal)) return false;
    string cred;
    try
    {
        cred = Encoding.UTF8.GetString(Convert.FromBase64String(nagl[6..].Trim()));
    }
    catch (FormatException)
    {
        return false;
    }
    var i = cred.IndexOf(':');
    return i >= 0 & Rowne(i >= 0 ? cred[..i] : cred, login) & Rowne(i >= 0 ? cred[(i + 1)..] : "", haslo);
}

IResult Odpowiedz(int kod, string tresc, string typ = "application/json") =>
    Results.Content(tresc, typ + "; charset=utf-8", Encoding.UTF8, kod);

IResult Odmowa(HttpContext ctx)
{
    ctx.Response.Headers.WWWAuthenticate = "Basic realm=\"RCP\"";
    return Odpowiedz(401, "{\"error\":\"unauthorized\"}");
}

var builder = WebApplication.CreateBuilder(args);
builder.WebHost.UseUrls($"http://0.0.0.0:{port}");
builder.Logging.SetMinimumLevel(LogLevel.Warning);
var app = builder.Build();

// Wyjatek w obsludze zadania -> 500 {"error":"internal"}, serwer dziala dalej.
// BadHttpRequestException (zawieszony albo zepsuty klient) obsluguje Kestrel sam.
app.Use(async (ctx, nastepny) =>
{
    try
    {
        await nastepny(ctx);
    }
    catch (Exception e) when (e is not BadHttpRequestException && !ctx.Response.HasStarted)
    {
        Console.Error.WriteLine(e);
        ctx.Response.Clear();
        ctx.Response.StatusCode = 500;
        ctx.Response.ContentType = "application/json; charset=utf-8";
        ctx.Response.Headers.Connection = "close";
        await ctx.Response.WriteAsync("{\"error\":\"internal\"}");
    }
});
app.UseRouting();   // po obsludze wyjatkow - tez bledy routingu daja 500

// POST na dowolna sciezke - czytnik wysyla na adres z ustawien, np. /odbicia.
app.MapPost("/{**sciezka}", async (HttpContext ctx) =>
{
    // Cialo czytane przed sprawdzeniem hasla - inaczej zostaloby w polaczeniu keep-alive.
    var buf = new byte[CialoMax + 1];
    int n = 0, r;
    while (n < buf.Length && (r = await ctx.Request.Body.ReadAsync(buf.AsMemory(n))) > 0) n += r;
    if (n > CialoMax) return Odpowiedz(413, "{\"error\":\"body\"}");
    if (!Zalogowany(ctx.Request)) return Odmowa(ctx);
    // Formularz tylko z ciala - query string w adresie jest pomijany.
    var (kod, tresc) = Przyjmij(buf.AsSpan(0, n));
    return Odpowiedz(kod, tresc);
});

IResult GetRaport(HttpContext ctx)
{
    if (!Zalogowany(ctx.Request)) return Odmowa(ctx);
    // zly query string = biezacy miesiac, nigdy 400
    var miesiac = ctx.Request.Query["miesiac"].ToString();
    if (!Regex.IsMatch(miesiac, @"^[0-9]{4}-(0[1-9]|1[0-2])\z"))
        miesiac = Lokalny(DateTimeOffset.UtcNow.ToUnixTimeSeconds(), "yyyy-MM");
    return Odpowiedz(200, Raport(miesiac), "text/html");
}
app.MapGet("/", GetRaport);
app.MapGet("/raport", GetRaport);

Console.WriteLine($"Serwer odbic na porcie {port}, odbic w {plikOdbic}: {odbicia.Count}");
app.Run();
return 0;

record Odbicie(long Epoch, string Rfid, string Machine);

static class Tekst
{
    // Pierwsze `max` znakow Unicode (code pointow, jak w Pythonie), bez rozcinania
    // par zastepczych UTF-16.
    public static string Pierwsze(string s, int max)
    {
        var i = 0;
        for (var n = 0; n < max && i < s.Length; n++) i += char.IsSurrogatePair(s, i) ? 2 : 1;
        return s[..i];
    }

    // Bajty pliku -> linie: zle bajty UTF-8 -> U+FFFD, linie tylko po '\n', '\r' na koncu
    // linii (CRLF) zdejmowany. bom: zdejmuje BOM (U+FEFF) z poczatku pliku.
    public static IEnumerable<string> Linie(byte[] dane, bool bom = false)
    {
        var s = Encoding.UTF8.GetString(dane);
        if (bom && s.StartsWith('\uFEFF')) s = s[1..];
        return s.Split('\n').Select(l => l.EndsWith('\r') ? l[..^1] : l);
    }

    // Cialo formularza jak parse_qs w Pythonie: pola po '&', nazwa=wartosc,
    // pierwsza wartosc pola wygrywa.
    public static Dictionary<string, string> Formularz(ReadOnlySpan<byte> cialo)
    {
        var pola = new Dictionary<string, string>(StringComparer.Ordinal);
        while (true)
        {
            var i = cialo.IndexOf((byte)'&');
            var para = i < 0 ? cialo : cialo[..i];
            if (!para.IsEmpty)
            {
                var j = para.IndexOf((byte)'=');
                pola.TryAdd(Odkoduj(j < 0 ? para : para[..j]), j < 0 ? "" : Odkoduj(para[(j + 1)..]));
            }
            if (i < 0) return pola;
            cialo = cialo[(i + 1)..];
        }
    }

    // '+' = spacja, %XX -> bajt (bledne % zostaje doslownie), bajty jako UTF-8,
    // zle sekwencje -> U+FFFD (jak Python errors="replace").
    static string Odkoduj(ReadOnlySpan<byte> s)
    {
        var b = new byte[s.Length];
        var n = 0;
        for (var i = 0; i < s.Length; i++)
        {
            int h, l;
            if (s[i] == '%' && i + 2 < s.Length && (h = Hex(s[i + 1])) >= 0 && (l = Hex(s[i + 2])) >= 0)
            {
                b[n++] = (byte)(h << 4 | l);
                i += 2;
            }
            else b[n++] = s[i] == '+' ? (byte)' ' : s[i];
        }
        return Encoding.UTF8.GetString(b, 0, n);
    }

    static int Hex(byte c) => c switch
    {
        >= (byte)'0' and <= (byte)'9' => c - '0',
        >= (byte)'a' and <= (byte)'f' => c - 'a' + 10,
        >= (byte)'A' and <= (byte)'F' => c - 'A' + 10,
        _ => -1,
    };
}

static class Szablon
{
    public const string Html = """
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
}
