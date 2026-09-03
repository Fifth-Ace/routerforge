package main

import "testing"

func TestCryptVectors(t *testing.T) {
	tests := []struct{ hash, pass string }{
		{"$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1", "Hello world!"},
		{"$6$rounds=10000$saltstringsaltst$OW1/O6BYHV6BcXZu8QVeXbDWra3Oeqh0sbHbbMCVNSnCM/UrjmM0Dp8vOuZeHBy/YTBmSK6H9qs/y3RnOaw5v.", "Hello world!"},
		{"$5$saltstring$5B8vYYiY.CVt1RlTTf8KbXBH3hsxY/GNooZaBBGWEc5", "Hello world!"},
		{"$1$deadbeef$Q7g0UO4hRC0mgQUQ/qkjZ0", "password"},
		{"$1$$pL/BYSxMXs.jVuSV1lynn1", "abcdefghijk"},
	}
	for _, tt := range tests {
		ok, err := verifyUnixCrypt(tt.hash, tt.pass)
		if err != nil || !ok {
			t.Fatalf("%s err=%v ok=%v", tt.hash, err, ok)
		}
		ok, _ = verifyUnixCrypt(tt.hash, tt.pass+"x")
		if ok {
			t.Fatal("bad pass accepted")
		}
	}
}
