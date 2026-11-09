# Inny język

Protokół jest w pełni opisany w [specyfikacji ADARAZ Link](./protocol).
W skrócie:

1. **Transport:** WebSocket pod `/api/adaraz/ws`, podprotokół `adaraz.v1`, tylko
   wiadomości binarne, jedna wiadomość WebSocket = jedna wiadomość Noise (do 4096 B).
2. **Parowanie:** `Noise_XXpsk3_25519_AESGCM_SHA256`, PSK wyliczany z 12-znakowego
   kodu (HKDF-SHA256). Serwer zapamiętuje klucz publiczny czytnika, czytnik klucz serwera.
3. **Sesja:** `Noise_IK_25519_AESGCM_SHA256`. Czytnik wysyła `hello`, serwer
   odpowiada `welcome` z kursorem, od którego czytnik ma wysyłać odbicia.
4. **Odbicia:** wiadomości `punches` (do 100 pozycji `[indeks, uid_hex, unix_ts]`),
   serwer odpowiada `ack` dopiero po trwałym zapisie.
5. **Przekaźnik:** `relay` -> `relay_result`, tylko za zgodą administratora czytnika.

Do sprawdzenia własnej implementacji użyj symulowanego czytnika z biblioteki
`adaraz` ([Test bez czytnika](./python#test)): to niezależna, przetestowana implementacja strony czytnika.
