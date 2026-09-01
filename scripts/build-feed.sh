#!/bin/sh
set -eu
DIR="${1:-.}"
OUT="$DIR/Packages"
: > "$OUT"

for IPK in "$DIR"/*.ipk; do
    [ -f "$IPK" ] || continue

    CONTROL="$(tar -xzOf "$IPK" ./control.tar.gz | tar -xzOf - ./control)"
    printf '%s\n' "$CONTROL" >> "$OUT"
    printf 'Filename: %s\n' "$(basename "$IPK")" >> "$OUT"
    printf 'Size: %s\n' "$(wc -c < "$IPK" | tr -d ' ')" >> "$OUT"
    if command -v md5sum >/dev/null 2>&1; then
        printf 'MD5sum: %s\n' "$(md5sum "$IPK" | awk '{print $1}')" >> "$OUT"
    fi
    if command -v sha256sum >/dev/null 2>&1; then
        printf 'SHA256sum: %s\n' "$(sha256sum "$IPK" | awk '{print $1}')" >> "$OUT"
    fi
    printf '\n' >> "$OUT"
done

gzip -9 -c "$OUT" > "$DIR/Packages.gz"
printf 'Created %s and %s\n' "$OUT" "$DIR/Packages.gz"
