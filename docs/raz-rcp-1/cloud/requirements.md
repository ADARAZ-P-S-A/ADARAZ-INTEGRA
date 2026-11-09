# Wymagania

::: warning Certyfikat krótkotrwały (shortlived)
Czytnik przyjmuje `https://` tylko z certyfikatem **ważnym najwyżej 7 dni**
(od `notBefore` do `notAfter`). Zwykły certyfikat 90-dniowy zostanie odrzucony,
a panel czytnika pokaże: `Chmura: certyfikat serwera nie jest krotkotrwaly (wymagany do 7 dni)`.
:::

## Adres `https://`

- certyfikat ważny najwyżej 7 dni, np. Let's Encrypt z profilem `shortlived`
  (certyfikaty na 160 godzin),
- wydany przez publiczny urząd certyfikacji (lista Mozilla, wbudowana w czytnik):
  certyfikat z własnego CA albo samopodpisany nie przejdzie,
- serwer wysyła pełny łańcuch (z certyfikatem pośrednim), czytnik go nie dociąga,
- w łańcuchu żadnego podpisu SHA-1 ani MD5,
- nazwa w certyfikacie zgodna z adresem,
- TLS 1.2 (czytnik nie używa TLS 1.3),
- automatyczne odnawianie: certyfikat żyje kilka dni, więc odnowienie i przeładowanie
  serwera muszą działać bez człowieka.

Co panel czytnika pokazuje przy odrzuconym certyfikacie (dosłownie, bez polskich znaków):

| Komunikat | Przyczyna |
|---|---|
| `Chmura: certyfikat serwera nie jest krotkotrwaly (wymagany do 7 dni)` | certyfikat ważny dłużej niż 7 dni |
| `Chmura: certyfikat serwera wygasl (albo zegar urzadzenia jest zly)` | wygasł albo czytnik ma zły czas |
| `Chmura: certyfikat serwera jeszcze niewazny - sprawdz zegar urzadzenia` | czas czytnika przed `notBefore` |
| `Chmura: certyfikat serwera podpisany SHA-1 albo MD5` | słaby podpis w łańcuchu |
| `Chmura: certyfikat serwera wystawiony kluczem CA, ktory wyciekl - podrobiony` | klucz CA z listy kluczy, które wyciekły |

## Adres `http://`

Bez szyfrowania działa tylko:

- do **adresu IP w sieci lokalnej**: 10.x, 172.16-31.x, 192.168.x, 169.254.x,
  IPv6 fc00::/7 i fe80::/10; nie do nazwy hosta, nie do 127.0.0.1,
- i nigdy przez otwartą sieć Wi-Fi.

Czytnik ostrzega wtedy, że kod parowania, hasło, odbicia i lista pracowników idą
jawnym tekstem. Używaj tego tylko w zamkniętej sieci (osobny VLAN, VPN).

## Adres serwera

- tylko zwykłe znaki ASCII, bez `@`, `?` i `#`,
- adres + `/device/v1/punch` krótszy niż 128 znaków,
- przekierowania: najwyżej 3 i tylko na `https://`,
- odpowiedź w ciągu 8 sekund,
- adres można zmienić w czytniku tylko wtedy, gdy nie jest połączony.

## Jak dostać certyfikat shortlived

certbot 4.0 lub nowszy:

```bash
certbot certonly --preferred-profile shortlived --webroot -w /var/www/html -d rcp.example.com
```

Caddy (profil ustawiany w wystawcy ACME; sprawdź w dokumentacji swojej wersji Caddy):

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

Źródła: [Let's Encrypt: Certbot i krótsze certyfikaty](https://letsencrypt.org/2026/03/11/shorter-certs-certbot),
[forum Caddy](https://caddy.community/t/shortlived-certificates-from-lets-encrypt/33399).
