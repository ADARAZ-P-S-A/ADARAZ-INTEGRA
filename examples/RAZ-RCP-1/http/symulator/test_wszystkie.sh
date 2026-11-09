#!/usr/bin/env bash
# Test zgodnosci wszystkich przykladow / conformance test of all examples.
#
#   symulator/test_wszystkie.sh                 # wszystkie, brakujace srodowiska pomijane
#   symulator/test_wszystkie.sh python go       # wybrane
#   WYMAGAJ=1 symulator/test_wszystkie.sh       # brak srodowiska = blad (CI)
#
# Kazdy serwer startuje w osobnym katalogu tymczasowym dwa razy: z czasem UTC
# (format http3) i z czasem polskim (format Skryba 10 bit).
set -u

KORZEN=$(cd "$(dirname "$0")/.." && pwd)
SYM="$KORZEN/symulator/symulator_czytnika.py"
TMP=$(mktemp -d)
PIDS=()
BLEDY=0
PORT=18700

sprzatanie() {
    for p in "${PIDS[@]}"; do kill "$p" 2>/dev/null; done
    wait 2>/dev/null
    rm -rf "$TMP"
}
trap sprzatanie EXIT

jest() { command -v "$1" >/dev/null 2>&1; }

# Przygotowanie (kompilacja) - raz na jezyk. Zwraca 1, gdy brak srodowiska.
przygotuj() {
    case $1 in
        python) jest python3 ;;
        go) jest go && (cd "$KORZEN/go" && go build -o "$TMP/rcp-go" .) ;;
        nodejs) jest node ;;
        java) jest java ;;
        csharp) jest dotnet || return 1
                dotnet build "$KORZEN/csharp" -o "$TMP/cs" -v q --nologo >"$TMP/cs.log" 2>&1 \
                    || { cat "$TMP/cs.log"; return 1; } ;;
        php) jest php ;;
    esac
}

# Serwer zamiast biezacego procesu (exec - PID w tle to PID serwera).
uruchom() {
    case $1 in
        python) exec python3 "$KORZEN/python/server.py" ;;
        go) exec "$TMP/rcp-go" ;;
        nodejs) exec node "$KORZEN/nodejs/server.js" ;;
        java) exec java "$KORZEN/java/RcpServer.java" ;;
        csharp) exec dotnet "$TMP/cs/RcpServer.dll" ;;
        php) exec php -S "127.0.0.1:$RCP_PORT" "$KORZEN/php/index.php" ;;
    esac
}

czekaj_na_port() {
    for _ in $(seq 1 120); do
        python3 - "$1" <<'EOF' && return 0
import socket, sys
try:
    socket.create_connection(("127.0.0.1", int(sys.argv[1])), timeout=1).close()
except OSError:
    sys.exit(1)
EOF
        sleep 0.5
    done
    return 1
}

test_serwera() {        # jezyk strefa_czytnika format
    local jezyk=$1 strefa=$2 format=$3 dir="$TMP/$1-$3"
    PORT=$((PORT + 1))
    mkdir -p "$dir"
    (cd "$dir" && export RCP_PORT=$PORT RCP_HASLO=test-haslo RCP_STREFA_CZYTNIKA=$strefa \
        RCP_DANE="$dir/odbicia.tsv" RCP_PRACOWNICY="$dir/pracownicy.csv" && uruchom "$jezyk") \
        >"$dir/serwer.log" 2>&1 &
    PIDS+=($!)
    if ! czekaj_na_port "$PORT"; then
        echo "BLAD $jezyk: serwer nie wystartowal"; cat "$dir/serwer.log"; BLEDY=$((BLEDY + 1)); return
    fi
    echo "== $jezyk ($format, czas czytnika $strefa)"
    if ! python3 "$SYM" "http://127.0.0.1:$PORT/odbicia" --haslo test-haslo --format "$format" --test; then
        BLEDY=$((BLEDY + 1)); cat "$dir/serwer.log"
    fi
    kill "${PIDS[-1]}" 2>/dev/null
}

JEZYKI=("$@")
[ ${#JEZYKI[@]} -eq 0 ] && JEZYKI=(python go nodejs java csharp php)

for j in "${JEZYKI[@]}"; do
    if ! przygotuj "$j"; then
        if [ "${WYMAGAJ:-0}" = 1 ]; then echo "BLAD $j: brak srodowiska albo kompilacja nieudana"; BLEDY=$((BLEDY + 1))
        else echo "-- $j pominiety (brak srodowiska)"; fi
        continue
    fi
    test_serwera "$j" UTC http3
    test_serwera "$j" Europe/Warsaw skryba10
done

echo
[ $BLEDY -eq 0 ] && echo "WSZYSTKO OK" || echo "BLEDOW: $BLEDY"
exit $((BLEDY > 0))
