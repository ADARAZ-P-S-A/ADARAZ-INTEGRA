# Ustawienia czytnika

W panelu czytnika: **Ustawienia > Integracje > Synchronizacja**:

| Pole | Wartość |
|---|---|
| Wysyłaj odbicia na serwer | włączone |
| Format | `http3 - UID jako hex` |
| Adres serwera (URL) | `http://<IP komputera>:8080/odbicia` |
| Login / Hasło | `rcp` / wartość `RCP_HASLO` |
| ID urządzenia (machineId) | dowolna nazwa, np. `Brama` |
| Interwał [min] | 1 do testów, fabrycznie 15 |

Opcja „Usuwaj odbicia po wysłaniu” sprawia, że odbicie przyjęte przez serwer
(`200`) znika z czytnika. Bez niej zostaje w czytniku oznaczone jako wysłane.

## Adres serwera i HTTPS

- `http://` działa tylko do serwera w sieci lokalnej (10.x, 172.16-31.x,
  192.168.x, 169.254.x, IPv6 fc/fd/fe80 albo nazwa wskazująca taki adres)
  i nigdy przez otwartą sieć Wi-Fi.
- Do serwera w internecie użyj `https://` z certyfikatem publicznego urzędu
  certyfikacji (np. Let's Encrypt), np. za nginx albo Caddy.
- W adresie używaj zwykłych znaków (litery, cyfry, `- . _ ~ /`). Niektóre serwery
  HTTP odpowiadają `400` na adres z luźnym `%`, a takie odbicie czytnik porzuca.
