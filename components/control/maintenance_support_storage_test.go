package main

import "testing"

func TestSupportStorageMonitoringIgnoresImmutableAndPseudoFilesystems(t *testing.T) {
	cases := []struct {
		item storageInfo
		want bool
	}{
		{storageInfo{FSType: "ext4", TotalBytes: 1024}, true},
		{storageInfo{FSType: "ubifs", TotalBytes: 1024}, true},
		{storageInfo{FSType: "squashfs", TotalBytes: 1024}, false},
		{storageInfo{FSType: "proc", TotalBytes: 0}, false},
		{storageInfo{FSType: "sysfs", TotalBytes: 0}, false},
		{storageInfo{FSType: "tmpfs", TotalBytes: 1024}, false},
		{storageInfo{FSType: "ext4", TotalBytes: 0}, false},
	}
	for _, tc := range cases {
		if got := adminSupportStorageMonitored(tc.item); got != tc.want {
			t.Fatalf("%s total=%d: got %v want %v", tc.item.FSType, tc.item.TotalBytes, got, tc.want)
		}
	}
}
