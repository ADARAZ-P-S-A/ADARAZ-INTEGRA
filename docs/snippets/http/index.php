<?php
// Minimal RAZ-RCP-1 punch receiver (PHP 8.1+). Run: RCP_HASLO=secret php -S 0.0.0.0:8080 index.php
// Reader: Settings > Integrations > Synchronization, format http3,
// URL http://<this computer>:8080/odbicia, login rcp, password = RCP_HASLO.
// Under Apache/nginx keep odbicia.tsv outside the web root.
$login = 'rcp';
$haslo = getenv('RCP_HASLO');
$file = __DIR__ . '/odbicia.tsv';

function answer(int $code, string $body): never {
    http_response_code($code);
    header('Content-Type: application/json');
    exit($body);
}

$given = ($_SERVER['PHP_AUTH_USER'] ?? '') . ':' . ($_SERVER['PHP_AUTH_PW'] ?? '');
if (!hash_equals("$login:$haslo", $given)) {
    answer(401, '{"error":"unauthorized"}');        // punch stays in the reader
}
$rfid = $_POST['rfidID'] ?? '';
$machine = str_replace(["\r", "\n", "\t"], ' ', $_POST['machineId'] ?? '');
$ts = DateTimeImmutable::createFromFormat('!Y-m-d H:i:s', $_POST['timestamp'] ?? '', new DateTimeZone('UTC'));
if (!preg_match('/^[0-9a-f]{1,20}$/', $rfid) || !$ts) {
    answer(400, '{"error":"bad request"}');         // dropped by the reader
}
if ((int)$ts->format('Y') < 2026) {
    answer(200, '{"stored":false,"reason":"clock"}');
}
$line = $ts->getTimestamp() . "\t$rfid\t$machine\n";

$f = fopen($file, 'c+');
flock($f, LOCK_EX);                                 // one request at a time
while (($l = fgets($f)) !== false) {
    if ($l === $line) {
        answer(200, '{"stored":false,"reason":"duplicate"}');
    }
}
fwrite($f, $line);                                  // store first...
fflush($f);
fsync($f);
answer(200, '{"stored":true}');                     // ...then answer 200 (exit releases the lock)
