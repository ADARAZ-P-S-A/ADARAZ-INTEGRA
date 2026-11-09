# RAZ-RCP-1

Czytnik kart zbliżeniowych i rejestrator czasu pracy. Każde przyłożenie karty
zapisuje jako odbicie: numer karty i czas. Odbicia może przekazać do Twojego
systemu na dwa sposoby.

## Wybierz sposób integracji

<div class="choice">
  <a class="choice-card" href="./http">
    <span class="choice-tag">najprostszy</span>
    <span class="choice-title">Wysyłanie odbić (HTTP)</span>
    <span class="choice-text">Czytnik sam wysyła każde odbicie zwykłym żądaniem POST na adres Twojego serwera. Po Twojej stronie wystarczy jeden endpoint HTTP w dowolnym języku.</span>
  </a>
  <a class="choice-card" href="./link">
    <span class="choice-tag">w obie strony</span>
    <span class="choice-title">ADARAZ Link</span>
    <span class="choice-text">Stałe, szyfrowane połączenie czytnika z Twoim serwerem: odbicia na bieżąco, stan czytnika i zdalne sterowanie przekaźnikiem (np. otwieranie drzwi).</span>
  </a>
</div>

## Porównanie

| | Wysyłanie odbić (HTTP) | ADARAZ Link |
|---|---|---|
| Kierunek | czytnik wysyła odbicia, serwer tylko odpowiada | stałe połączenie, wiadomości w obie strony |
| Co dostajesz | odbicia: karta, czas, ID czytnika | odbicia na bieżąco, stan czytnika, sterowanie przekaźnikiem |
| Kiedy odbicie dociera | co ustawiony interwał (od 1 minuty) | po chwili, zaraz po przyłożeniu karty |
| Po Twojej stronie | dowolny serwer HTTP, jeden endpoint `POST` | serwer WebSocket z protokołem Noise: gotowa biblioteka w Pythonie albo własna implementacja |
| Bezpieczeństwo | `https://` z certyfikatem publicznego urzędu albo `http://` w sieci lokalnej, login i hasło | wzajemne uwierzytelnienie kluczami i szyfrowanie Noise, także po `ws://` w sieci lokalnej |
| Parowanie | wpisujesz adres, login i hasło | jednorazowy kod z serwera, ważny 10 minut |
| Ustawienia w czytniku | Ustawienia > Integracje > Synchronizacja | Ustawienia > Integracje > Home Assistant i inne serwery |
| Dobre do | systemów RCP i kadrowych, które mają zbierać odbicia | kontroli dostępu, automatyki, Home Assistant, reakcji w czasie rzeczywistym |

Obie integracje mogą działać jednocześnie: czytnik może wysyłać odbicia przez
HTTP do systemu kadrowego i równocześnie być połączony przez ADARAZ Link
z serwerem automatyki.

## Wspólne zasady

- **Czytnik zawsze łączy się sam.** Twój serwer nigdy nie łączy się z czytnikiem,
  więc czytnik nie potrzebuje publicznego adresu ani otwartego portu.
- **Odbicie nie ginie.** Czytnik trzyma odbicia u siebie, dopóki serwer nie
  potwierdzi, że je zapisał. Przy braku sieci wysyła je później.
- **Czas jest w UTC** (poza starymi formatami Skryba w wysyłaniu HTTP).
- **Czytnik nie wie, czy to wejście, czy wyjście.** Wysyła samo odbicie; kierunek
  wylicza Twój system, np. łącząc odbicia osoby kolejno w pary.
