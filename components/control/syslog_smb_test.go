package main

import (
	"strings"
	"testing"
)

func TestP455KeeneticUSBMountAliasesAreOneDevice(t *testing.T) {
	mounts := []rfSyslogMount{
		{Point: "/opt", Device: "/dev/sda1", FS: "ext4"},
		{Point: "/tmp/mnt/24f0426e-c415-4210-81e9-3360fa9de595", Device: "/dev/sda1", FS: "ext4"},
	}
	got, usb := rfSyslogEntwareUSB(mounts)
	if !usb || got != "/dev/sda1" {
		t.Fatalf("Keenetic Entware USB not detected: %q %t", got, usb)
	}
	mounts[1].Device = "/dev/sdb1"
	if _, usb = rfSyslogEntwareUSB(mounts); usb {
		t.Fatal("different partition incorrectly aliased as Entware USB")
	}
	mounts = []rfSyslogMount{{Point: "/opt", Device: "/dev/mtdblock6", FS: "ubifs"}}
	if _, usb = rfSyslogEntwareUSB(mounts); usb {
		t.Fatal("internal NAND identified as USB")
	}
}
func TestP455SMBInputsGuarded(t *testing.T) {
	tests := []struct {
		host, share string
		valid       bool
	}{
		{`192.168.10.20`, "RouterLogs", true},
		{`\\192.168.10.20\RouterLogs`, "", true},
		{`//nas.example.lan/RouterLogs`, "", true},
		{`127.0.0.1`, "Logs", false},
		{`192.168.10.20;rm`, "Logs", false},
		{`192.168.10.20`, "../etc", false},
		{`192.168.10.20`, "Logs/Files", false},
		{`//192.168.10.20/Logs`, "Other", false},
	}
	for _, tt := range tests {
		_, _, err := rfSMBNormalize(tt.host, tt.share)
		if (err == nil) != tt.valid {
			t.Errorf("host=%q share=%q err=%v", tt.host, tt.share, err)
		}
	}
	req := rfSMBConnectRequest{Confirm: "CONNECT SYSLOG SMB", Username: "logs", Password: "secret"}
	if err := rfSMBCheckCredentials(req); err != nil {
		t.Fatal(err)
	}
	req.Password = "pass\noptions=unsafe"
	if err := rfSMBCheckCredentials(req); err == nil {
		t.Fatal("newline in credentials accepted")
	}
	req.Password = "secret"
	req.Confirm = ""
	if err := rfSMBCheckCredentials(req); err == nil {
		t.Fatal("missing explicit confirmation accepted")
	}
}
func TestP455AllowedRootsAndNoFallback(t *testing.T) {
	for _, p := range []string{"/opt/var/log/routerforge-syslog", rfSMBRoot, "/tmp/mnt/24f0426e-c415-4210-81e9-3360fa9de595/routerforge-syslog"} {
		if !rfSyslogRootAllowed(p) {
			t.Errorf("valid root rejected: %s", p)
		}
	}
	for _, p := range []string{"/tmp/routerforge-syslog", "/opt/mnt/a/../../etc", "/tmp/mnt/a/not-our-log-dir", "/opt/mnt/a/../routerforge-syslog"} {
		if rfSyslogRootAllowed(p) {
			t.Errorf("unsafe path accepted: %s", p)
		}
	}
	if strings.Contains(rfSMBPoint, "/tmp/") {
		t.Fatal("SMB mountpoint cannot depend on temporary pathname")
	}
}
