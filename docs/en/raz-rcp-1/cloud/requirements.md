# Requirements

::: warning Short-lived certificate
The reader accepts `https://` only with a certificate **valid for at most 7 days**
(from `notBefore` to `notAfter`). A regular 90-day certificate is rejected, and
the reader's panel shows: `Chmura: certyfikat serwera nie jest krotkotrwaly (wymagany do 7 dni)`.
:::

## `https://` address

- certificate valid for at most 7 days, e.g. Let's Encrypt with the `shortlived`
  profile (160-hour certificates),
- issued by a public certificate authority (the Mozilla list built into the reader):
  a certificate from your own CA or a self-signed one will not work,
- the server sends the full chain (with the intermediate certificate); the reader does not fetch it,
- no SHA-1 or MD5 signature in the chain,
- the name in the certificate matches the address,
- TLS 1.2 (the reader does not use TLS 1.3),
- automatic renewal: the certificate lives a few days, so renewal and reloading
  the server must work without a person.

What the reader's panel shows for a rejected certificate (verbatim, in Polish):

| Message | Cause |
|---|---|
| `Chmura: certyfikat serwera nie jest krotkotrwaly (wymagany do 7 dni)` | the certificate is valid for longer than 7 days |
| `Chmura: certyfikat serwera wygasl (albo zegar urzadzenia jest zly)` | expired, or the reader's clock is wrong |
| `Chmura: certyfikat serwera jeszcze niewazny - sprawdz zegar urzadzenia` | the reader's clock is before `notBefore` |
| `Chmura: certyfikat serwera podpisany SHA-1 albo MD5` | a weak signature in the chain |
| `Chmura: certyfikat serwera wystawiony kluczem CA, ktory wyciekl - podrobiony` | a CA key from the list of leaked keys |

## `http://` address

Without encryption it works only:

- to an **IP address in the local network**: 10.x, 172.16-31.x, 192.168.x,
  169.254.x, IPv6 fc00::/7 and fe80::/10; not to a host name, not to 127.0.0.1,
- and never over an open Wi-Fi network.

The reader then warns that the pairing code, the password, punches and the
employee list travel in plain text. Use it only in a closed network (a separate VLAN, a VPN).

## Server address

- plain ASCII characters only, no `@`, `?` or `#`,
- the address + `/device/v1/punch` shorter than 128 characters,
- redirects: at most 3, and only to `https://`,
- an answer within 8 seconds,
- the address can be changed on the reader only while it is disconnected.

## Getting a short-lived certificate

certbot 4.0 or newer:

```bash
certbot certonly --preferred-profile shortlived --webroot -w /var/www/html -d rcp.example.com
```

Caddy (the profile is set in the ACME issuer; check the documentation of your Caddy version):

```
rcp.example.com {
	tls {
		issuer acme {
			profile shortlived
		}
	}
	reverse_proxy 127.0.0.1:8080
}
```

Sources: [Let's Encrypt: Certbot and shorter certificates](https://letsencrypt.org/2026/03/11/shorter-certs-certbot),
[Caddy forum](https://caddy.community/t/shortlived-certificates-from-lets-encrypt/33399).
