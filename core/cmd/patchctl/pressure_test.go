package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaintenancePressureOK(t *testing.T) {
	if !maintenancePressureOK(hostPressure{Load1M: 1.5, AvailableMemoryMB: 4096}, 2, 1024) {
		t.Fatal("expected low pressure to pass")
	}
	if maintenancePressureOK(hostPressure{Load1M: 3, AvailableMemoryMB: 4096}, 2, 1024) {
		t.Fatal("expected high load to fail")
	}
	if maintenancePressureOK(hostPressure{Load1M: 1, AvailableMemoryMB: 512}, 2, 1024) {
		t.Fatal("expected low available memory to fail")
	}
}

func TestReadLoad1M(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loadavg")
	if err := os.WriteFile(path, []byte("1.23 0.50 0.25 1/200 12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	load, err := readLoad1M(path)
	if err != nil {
		t.Fatal(err)
	}
	if load != 1.23 {
		t.Fatalf("load = %.2f, want 1.23", load)
	}
}

func TestReadAvailableMemoryMB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meminfo")
	if err := os.WriteFile(path, []byte("MemTotal: 8192000 kB\nMemAvailable: 2097152 kB\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	memoryMB, err := readAvailableMemoryMB(path)
	if err != nil {
		t.Fatal(err)
	}
	if memoryMB != 2048 {
		t.Fatalf("available memory = %d, want 2048", memoryMB)
	}
}
