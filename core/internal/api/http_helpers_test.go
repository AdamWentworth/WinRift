package api

import (
	"encoding/json"
	"testing"
)

func TestQueryUint16RejectsOutOfRangeValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		value    string
		fallback uint16
		want     uint16
	}{
		{name: "valid", value: "420", fallback: 1, want: 420},
		{name: "trimmed", value: " 420 ", fallback: 1, want: 420},
		{name: "empty", fallback: 420, want: 420},
		{name: "negative", value: "-1", fallback: 420, want: 420},
		{name: "overflow", value: "65536", fallback: 420, want: 420},
		{name: "invalid", value: "ranked", fallback: 420, want: 420},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := queryUint16(test.value, test.fallback); got != test.want {
				t.Fatalf("queryUint16(%q, %d) = %d, want %d", test.value, test.fallback, got, test.want)
			}
		})
	}
}

func TestUint16FromAnyRejectsOutOfRangeValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		value any
		want  uint16
	}{
		{name: "int", value: int(420), want: 420},
		{name: "int overflow", value: int(65536)},
		{name: "int64 overflow", value: int64(65536)},
		{name: "uint64 overflow", value: uint64(65536)},
		{name: "float overflow", value: float64(65536)},
		{name: "json number overflow", value: json.Number("65536")},
		{name: "string overflow", value: "65536"},
		{name: "negative", value: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := uint16FromAny(test.value); got != test.want {
				t.Fatalf("uint16FromAny(%v) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}
