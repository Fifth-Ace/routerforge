#!/bin/sh

# RouterForge Entware build target profiles.
# Public release is currently aarch64-3.10 only.
# mips-3.4 / mipsel-3.4 are compile-smoke targets until hardware validation.

routerforge_target_init() {
    target="${1:-aarch64-3.10}"

    case "$target" in
        aarch64-3.10)
            RF_TARGET="aarch64-3.10"
            RF_OPKG_ARCH="aarch64-3.10"
            RF_GOARCH="arm64"
            RF_GOMIPS=""
            RF_ENTWARE_FEED="aarch64-k3.10"
            ;;
        mips-3.4)
            RF_TARGET="mips-3.4"
            RF_OPKG_ARCH="mips-3.4"
            RF_GOARCH="mips"
            RF_GOMIPS="softfloat"
            RF_ENTWARE_FEED="mipssf-k3.4"
            ;;
        mipsel-3.4)
            RF_TARGET="mipsel-3.4"
            RF_OPKG_ARCH="mipsel-3.4"
            RF_GOARCH="mipsle"
            RF_GOMIPS="softfloat"
            RF_ENTWARE_FEED="mipselsf-k3.4"
            ;;
        *)
            echo "unsupported RouterForge target: $target" >&2
            return 2
            ;;
    esac

    export RF_TARGET RF_OPKG_ARCH RF_GOARCH RF_GOMIPS RF_ENTWARE_FEED
}

routerforge_go() {
    if [ -n "${RF_GOMIPS:-}" ]; then
        CGO_ENABLED=0 GOOS=linux GOARCH="$RF_GOARCH" GOMIPS="$RF_GOMIPS" go "$@"
    else
        CGO_ENABLED=0 GOOS=linux GOARCH="$RF_GOARCH" go "$@"
    fi
}
