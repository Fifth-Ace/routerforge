package releasetransport

import (
	"net/url"
	"testing"
)

func TestCandidates(t *testing.T) {
	for _, tag := range []string{"routerforge-dev", "routerforge-beta", "routerforge-stable", "routerforge-v1.0.0"} {
		for _, asset := range []string{"routerforge-core_1-dev.r1458.af90aae4fb68_aarch64-3.10.ipk", "routerforge-stable-index-mipsel-3.4.json", "routerforge-beta-index-mips-3.4.json"} {
			got := Candidates(tag, asset)
			if len(got) != 2 || got[0] != GitHubBase+tag+"/"+asset || got[1] != ForgejoBase+tag+"/"+asset {
				t.Fatal(got)
			}
			if !IsMirror(got[1]) {
				t.Fatal(got)
			}
			for _, base := range []string{GitHubBase, LegacyBase} {
				a, b, ok := GitHubAsset(base + tag + "/" + asset)
				if !ok || a != tag || b != asset {
					t.Fatal(a, b, ok)
				}
			}
		}
	}
	for _, bad := range []string{"../x", "a/b", "a?x", "a#x", "%2e%2e", "a\\b", "a\n", ""} {
		if Candidates("routerforge-dev", bad) != nil || Candidates(bad, "index.json") != nil {
			t.Fatal(bad)
		}
	}
}
func TestRedirects(t *testing.T) {
	gh, _ := url.Parse(GitHubBase + "routerforge-dev/index.json")
	mirror, _ := url.Parse(ForgejoBase + "routerforge-dev/index.json")
	for _, v := range []struct {
		raw  string
		g, m bool
	}{
		{"https://release-assets.githubusercontent.com/signed?a=b", true, false},
		{"https://git.fifthace.ru/attachments/id", false, true},
		{"https://git.fifthace.ru.evil/a", false, false}, {"http://git.fifthace.ru/a", false, false}, {"https://user@git.fifthace.ru/a", false, false}, {"https://git.fifthace.ru:443/a", false, false},
	} {
		u, e := url.Parse(v.raw)
		if e != nil || allowedRedirect(u, gh) != v.g || allowedRedirect(u, mirror) != v.m {
			t.Fatal(v, e)
		}
	}
}
func TestMirrorIndexMatches(t *testing.T) {
	if !MirrorIndexMatches([]byte("same"), []byte("same")) || MirrorIndexMatches([]byte("old"), []byte("new")) || MirrorIndexMatches(nil, nil) {
		t.Fatal("invalid mirror freshness policy")
	}
}
