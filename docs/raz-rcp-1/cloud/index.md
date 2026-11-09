# Własny serwer chmury

Czytnik łączy się z Twoim serwerem dokładnie tak, jak z Chmurą ADARAZ. Twój
serwer implementuje API urządzenia Chmury ADARAZ (`/device/v1`), a czytnik:

- paruje się z nim jednorazowym kodem i sam ustawia wysyłanie odbić,
- pobiera z niego **listę pracowników i ich kart** (kogo wpuścić i jak liczyć wejścia),
- wysyła odbicia,
- pyta, **kto jest w środku**, więc osoba może wejść jednym czytnikiem, a wyjść innym,
- zgłasza **nieznane karty**, żeby można je było przypisać w Twoim systemie.

```
czytnik ── odbicia, nieznane karty ──────────▶ Twój serwer
        ◀── lista pracowników, kto jest w środku ─
```

W panelu czytnika to pole **Własny serwer chmury** w sekcji Chmura ADARAZ.
Czytnik zawsze łączy się sam, co ustawiony interwał (od 1 minuty); serwer nigdy
nie łączy się z czytnikiem.

::: warning Najpierw wymagania
Czytnik przyjmuje `https://` tylko z **certyfikatem krótkotrwałym (shortlived),
ważnym najwyżej 7 dni**. Zwykły certyfikat 90-dniowy nie zadziała.
Szczegóły: [Wymagania](./requirements).
:::

## Spis

| Strona | Co jest na niej |
|---|---|
| [Wymagania](./requirements) | certyfikat shortlived, `http://` w sieci lokalnej, zasady adresu |
| [Serwer wzorcowy](./server) | gotowy serwer w Go: uruchomienie, parowanie, pracownicy, odbicia, test |
| [Parowanie](./pairing) | `POST /claim`: kod -> login, hasło, adresy |
| [Lista pracowników](./users) | `GET /users`: format users.bin, CRC, `304` |
| [Odbicia i nieznane karty](./punches) | `POST /punch`, `POST /seen` |
| [Obecność](./presence) | `GET /presence`: kto jest w środku |
| [Odpowiedzi i błędy](./responses) | co czytnik robi z każdym statusem, `GET /hello` |

## Jeden cykl czytnika

```
1. POST /seen       nieznane karty z kolejki (zaraz po przyłożeniu)
2. GET  /users      najwyżej co 50 s: lista pracowników albo 304
3. POST /punch      do 32 odbić, jedno na żądanie, jednym połączeniem
4. GET  /presence   najwyżej co 20 s: kto jest w środku albo 304
```

Wszystkie trasy poza `/claim` wymagają HTTP Basic z loginem i hasłem z odpowiedzi
`/claim`. Po sparowaniu lokalna lista pracowników czytnika jest tylko do odczytu,
a odbicia przyjęte przez serwer znikają z czytnika.
