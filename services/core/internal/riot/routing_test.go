package riot

import "testing"

func TestRouting(t *testing.T) {
	if NormalizePlatform("na") != "NA1" {
		t.Fatal("NA alias failed")
	}
	region, err := RegionForPlatform("KR")
	if err != nil {
		t.Fatal(err)
	}
	if region != "ASIA" {
		t.Fatalf("region = %s", region)
	}
}

func TestParseRiotID(t *testing.T) {
	gameName, tagLine, err := ParseRiotID("Some Name#NA1")
	if err != nil {
		t.Fatal(err)
	}
	if gameName != "Some Name" || tagLine != "NA1" {
		t.Fatalf("got %q %q", gameName, tagLine)
	}
	if _, _, err := ParseRiotID("missing"); err == nil {
		t.Fatal("expected error")
	}
}
