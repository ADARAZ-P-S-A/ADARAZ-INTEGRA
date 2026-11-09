# Another language

The protocol is fully described in the [ADARAZ Link specification](./protocol).
In short:

1. **Transport:** WebSocket at `/api/adaraz/ws`, subprotocol `adaraz.v1`, binary
   messages only, one WebSocket message = one Noise message (up to 4096 bytes).
2. **Pairing:** `Noise_XXpsk3_25519_AESGCM_SHA256`, the PSK derived from the
   12-character code (HKDF-SHA256). The server stores the reader's public key,
   the reader stores the server's.
3. **Session:** `Noise_IK_25519_AESGCM_SHA256`. The reader sends `hello`, the
   server answers `welcome` with the cursor the reader sends punches from.
4. **Punches:** `punches` messages (up to 100 items `[index, uid_hex, unix_ts]`),
   the server answers `ack` only after storing them durably.
5. **Relay:** `relay` -> `relay_result`, only with the reader administrator's consent.

To test your implementation, use the simulated reader from the `adaraz`
library ([Testing without a reader](./python#test)): an independent, tested implementation of the reader side.
