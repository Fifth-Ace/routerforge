package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPackageForServiceUsesExactInitScriptOwnership(t *testing.T) {
	servicePath := filepath.Clean("/opt/etc/init.d/S99example")
	owners := map[string]packageOwnership{
		"example": {
			Package: packageInfo{Name: "example", Version: "1.2.3"},
			List:    "/opt/lib/opkg/info/example.list",
			Files: []string{
				"/opt/bin/exampled",
				servicePath,
			},
		},
		"other": {
			Package: packageInfo{Name: "other"},
			List:    "/opt/lib/opkg/info/other.list",
			Files:   []string{"/opt/etc/init.d/S99other"},
		},
	}

	owner, ok := packageForService(servicePath, owners)
	if !ok {
		t.Fatal("expected owner")
	}
	if owner.Package.Name != "example" {
		t.Fatalf("owner=%q want example", owner.Package.Name)
	}
}

func TestClassifyPackageEvidence(t *testing.T) {
	configs, logs := classifyPackageEvidence([]string{
		"/opt/etc/init.d/S99example",
		"/opt/etc/example/config.json",
		"/opt/etc/example/rules.conf",
		"/opt/var/log/example.log",
		"/opt/bin/exampled",
	}, "/opt/etc/init.d/S99example")

	if len(configs) != 2 {
		t.Fatalf("configs=%v", configs)
	}
	if len(logs) != 1 || logs[0] != "/opt/var/log/example.log" {
		t.Fatalf("logs=%v", logs)
	}
}

func TestInspectServiceCorrelatesPackageProcessAndListener(t *testing.T) {
	service := serviceInfo{
		ID:            "S99example",
		Name:          "example",
		Path:          "/opt/etc/init.d/S99example",
		Executable:    true,
		Running:       false,
		RunningSource: "process-match",
		ModifiedAt:    time.Unix(100, 0),
	}
	owners := map[string]packageOwnership{
		"example": {
			Package: packageInfo{
				Name:         "example",
				Version:      "1.2.3",
				Architecture: "aarch64-3.10",
				Status:       "install ok installed",
			},
			List: "/opt/lib/opkg/info/example.list",
			Files: []string{
				"/opt/etc/init.d/S99example",
				"/opt/bin/exampled",
				"/opt/etc/example/config.json",
				"/opt/var/log/example.log",
			},
		},
	}
	processes := []processInfo{
		{
			PID:      123,
			Name:     "exampled",
			Command:  "/opt/bin/exampled",
			State:    "S",
			User:     "root",
			UID:      0,
			RSSKB:    2048,
			VmSizeKB: 4096,
			Threads:  3,
		},
		{
			PID:     456,
			Name:    "unrelated",
			Command: "/opt/bin/unrelated",
		},
	}
	ports := []portInfo{
		{
			Protocol:     "tcp4",
			LocalAddress: "0.0.0.0",
			LocalPort:    8080,
			State:        "LISTEN",
			PID:          123,
			Process:      "exampled",
			Inode:        "999",
		},
		{
			Protocol:     "tcp4",
			LocalAddress: "0.0.0.0",
			LocalPort:    9999,
			State:        "LISTEN",
			PID:          456,
		},
	}

	item := inspectService(service, owners, processes, ports)

	if item.Package == nil || item.Package.Name != "example" {
		t.Fatalf("package=%+v", item.Package)
	}
	if !item.Running || item.RunningSource != "service-inspector-process-evidence" {
		t.Fatalf("running=%v source=%q", item.Running, item.RunningSource)
	}
	if len(item.Processes) != 1 || item.Processes[0].PID != 123 {
		t.Fatalf("processes=%+v", item.Processes)
	}
	if len(item.ListeningPorts) != 1 || item.ListeningPorts[0].LocalPort != 8080 {
		t.Fatalf("ports=%+v", item.ListeningPorts)
	}
	if item.Resource.ProcessCount != 1 ||
		item.Resource.RSSKB != 2048 ||
		item.Resource.VmSizeKB != 4096 ||
		item.Resource.Threads != 3 ||
		item.Resource.Listeners != 1 {
		t.Fatalf("resource=%+v", item.Resource)
	}
	if len(item.ConfigPaths) != 1 || item.ConfigPaths[0] != "/opt/etc/example/config.json" {
		t.Fatalf("configs=%v", item.ConfigPaths)
	}
	if len(item.LogPaths) != 1 || item.LogPaths[0] != "/opt/var/log/example.log" {
		t.Fatalf("logs=%v", item.LogPaths)
	}
	if item.Evidence.PackageSource != "opkg-info-list" {
		t.Fatalf("package evidence=%q", item.Evidence.PackageSource)
	}
	if item.Evidence.PortSource != "proc-net+fd-inode" {
		t.Fatalf("port evidence=%q", item.Evidence.PortSource)
	}
}

func TestProcessMatchIsExactAfterNormalization(t *testing.T) {
	processes := []processInfo{
		{PID: 1, Name: "foo-bar", Command: "/opt/bin/foo-bar"},
		{PID: 2, Name: "foo-bar-helper", Command: "/opt/bin/foo-bar-helper"},
	}

	matched, evidence := matchServiceProcesses(processes, []string{"foobar"})
	if len(matched) != 1 || matched[0].PID != 1 {
		t.Fatalf("matched=%+v", matched)
	}
	if len(evidence) != 1 {
		t.Fatalf("evidence=%v", evidence)
	}
}
