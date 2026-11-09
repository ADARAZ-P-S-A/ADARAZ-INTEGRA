# RAZ-RCP-1

Czytnik kart zbliżeniowych i rejestrator czasu pracy. Każde przyłożenie karty
zapisuje jako odbicie: numer karty i czas. Z Twoim systemem może się połączyć
na trzy sposoby.

## Wybierz sposób integracji

<div class="choice">
  <a class="choice-card" href="./http/">
    <span class="choice-tag">najprostszy</span>
    <span class="choice-title">Wysyłanie odbić (HTTP)</span>
    <span class="choice-text">Czytnik sam wysyła każde odbicie zwykłym żądaniem POST na adres Twojego serwera. Po Twojej stronie wystarczy jeden endpoint HTTP w dowolnym języku.</span>
  </a>
  <a class="choice-card" href="./cloud/">
    <span class="choice-tag">jak Chmura ADARAZ</span>
    <span class="choice-title">Własny serwer chmury</span>
    <span class="choice-text">Twój serwer zamiast Chmury ADARAZ: parowanie kodem, lista pracowników wysyłana do czytników, odbicia, kto jest w środku i nieznane karty. Gotowy serwer wzorcowy w Go.</span>
  </a>
  <a class="choice-card" href="./link/">
    <span class="choice-tag">na żywo, w obie strony</span>
    <span class="choice-title">ADARAZ Link</span>
    <span class="choice-text">Stałe, szyfrowane połączenie czytnika z Twoim serwerem: odbicia na bieżąco, stan czytnika i zdalne sterowanie przekaźnikiem (np. otwieranie drzwi).</span>
  </a>
</div>

## Porównanie

| | Wysyłanie odbić (HTTP) | Własny serwer chmury | ADARAZ Link |
|---|---|---|---|
| Co dostajesz | odbicia: karta, czas, ID czytnika | odbicia, nieznane karty; czytnik dostaje od Ciebie listę pracowników i to, kto jest w środku | odbicia na bieżąco, stan czytnika, sterowanie przekaźnikiem |
| Lista pracowników w czytniku | ustawiana w czytniku | z Twojego serwera | ustawiana w czytniku |
| Kiedy odbicie dociera | co ustawiony interwał (od 1 minuty) | co ustawiony interwał (od 1 minuty) | zaraz po przyłożeniu karty |
| Po Twojej stronie | jeden endpoint `POST` | API Chmury ADARAZ (`/device/v1`): 6 tras | serwer WebSocket z protokołem Noise: gotowa biblioteka w Pythonie |
| Bezpieczeństwo | `https://` (publiczny certyfikat) albo `http://` w sieci lokalnej, login i hasło | `https://` z **certyfikatem shortlived (do 7 dni)** albo `http://` na adres IP w sieci lokalnej | wzajemne uwierzytelnienie kluczami i szyfrowanie Noise, także po `ws://` |
| Parowanie | wpisujesz adres, login i hasło | adres serwera i jednorazowy kod (8 znaków) | adres serwera i jednorazowy kod (12 znaków) |
| Ustawienia w czytniku | Ustawienia > Integracje > Synchronizacja | Ustawienia > Integracje > Chmura ADARAZ | Ustawienia > Integracje > Home Assistant i inne serwery |
| Dobre do | systemów, które mają tylko zbierać odbicia | systemów RCP i kadrowych, które zarządzają pracownikami i kartami | kontroli dostępu, automatyki, Home Assistant |

Wysyłanie odbić (HTTP) i własny serwer chmury się wykluczają: po sparowaniu
z serwerem chmury to on przejmuje ustawienia synchronizacji czytnika.
ADARAZ Link może działać obok każdej z nich.

## Wspólne zasady

- **Czytnik zawsze łączy się sam.** Twój serwer nigdy nie łączy się z czytnikiem,
  więc czytnik nie potrzebuje publicznego adresu ani otwartego portu.
- **Odbicie nie ginie.** Czytnik trzyma odbicia u siebie, dopóki serwer nie
  potwierdzi, że je zapisał. Przy braku sieci wysyła je później.
- **Czas jest w UTC** (poza starymi formatami Skryba w wysyłaniu HTTP).
- **Czytnik nie wie, czy to wejście, czy wyjście.** Wysyła samo odbicie; kierunek
  wylicza Twój system, np. łącząc odbicia osoby kolejno w pary.
