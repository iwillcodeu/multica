package userdisplay

import "testing"

func TestDisplayNameLengthUnits(t *testing.T) {
	if u := DisplayNameLengthUnits("abc"); u != 3 {
		t.Fatalf("ascii: got %d", u)
	}
	if u := DisplayNameLengthUnits("中文"); u != 4 {
		t.Fatalf("han: got %d", u)
	}
	if u := DisplayNameLengthUnits("a中"); u != 3 {
		t.Fatalf("mixed: got %d", u)
	}
}

func TestTruncateToMaxUnits(t *testing.T) {
	if got := TruncateToMaxUnits("abcdefghijklm"); got != "abcdefghijkl" {
		t.Fatalf("got %q", got)
	}
	if got := TruncateToMaxUnits("中文中文中文"); got != "中文中文中文" {
		t.Fatalf("got %q", got)
	}
}
