package main

import (
    "os"
    "strings"
    "testing"
)

func TestOdinKernelWithoutCommentCompatibility(t *testing.T) {
    body, err := os.ReadFile("../packaging/odin-engine.sh")
    if err != nil { t.Fatal(err) }
    s := string(body)
    if strings.Contains(s, "-m comment") { t.Fatal("odin must not require xt_comment") }
    for _, want := range []string{
        "owned KNOCKING-FIRST && owned KNOCKING-LAST",
        "-p tcp --dport 0 -j RETURN",
        "-p udp --dport 0 -j RETURN",
        "mark_owned KNOCKING-FIRST",
        "mark_owned KNOCKING-LAST",
        "FOREIGN_CHAIN_COLLISION_OR_LEGACY_SIGNATURE",
        "ODIN_RULE_APPEND_FAILED",
    } {
        if !strings.Contains(s, want) { t.Fatalf("missing odin compatibility contract %q", want) }
    }
}