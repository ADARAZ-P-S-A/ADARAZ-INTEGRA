# ADARAZ Link

Stałe, szyfrowane połączenie czytnika z Twoim serwerem. Czytnik sam łączy się
z serwerem przez WebSocket i trzyma połączenie otwarte:

- odbicia docierają zaraz po przyłożeniu karty, a te z czasu bez sieci są
  dosyłane później i oznaczone jako spóźnione,
- serwer widzi stan czytnika (połączony, czy czytnik kart działa, stan przekaźnika),
- serwer może sterować przekaźnikiem, np. otwierać drzwi, jeśli administrator
  czytnika na to pozwoli.

```
czytnik ══ WebSocket + Noise (łączy się sam) ══ Twój serwer
   odbicia, stan ─────────────────────────────▶
                 ◀───────────────────────────── potwierdzenia,
                                                przekaźnik
```

Każde połączenie to sesja [Noise](https://noiseprotocol.org/) ze wzajemnym
uwierzytelnieniem kluczami wymienionymi przy parowaniu, więc zwykłe `ws://`
w sieci lokalnej jest bezpieczne. Dla serwera w internecie działa też `wss://`.

## Trzy drogi

| | Dla kogo |
|---|---|
| [Home Assistant](./home-assistant) | gotowa integracja, bez pisania kodu |
| [Własny serwer w Pythonie](./python) | biblioteka `adaraz` robi cały protokół, Ty piszesz tylko obsługę odbić |
| [Inny język](./other-languages) | własna implementacja według specyfikacji |
