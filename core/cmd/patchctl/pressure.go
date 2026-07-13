package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type hostPressure struct {
	Load1M            float64
	AvailableMemoryMB int
}

func waitForMaintenancePressure(ctx context.Context, phase string) error {
	maxLoad := envFloat("PATCHCTL_MAX_LOAD_1M", 0)
	minMemoryMB := envInt("PATCHCTL_MIN_AVAILABLE_MEMORY_MB", 0)
	if maxLoad <= 0 && minMemoryMB <= 0 {
		return nil
	}

	attempts := envInt("PATCHCTL_PRESSURE_CHECK_ATTEMPTS", 60)
	if attempts <= 0 {
		attempts = 1
	}
	sleepSeconds := envInt("PATCHCTL_PRESSURE_CHECK_SLEEP_SECONDS", 10)
	if sleepSeconds <= 0 {
		sleepSeconds = 1
	}

	var last hostPressure
	for attempt := 1; attempt <= attempts; attempt++ {
		pressure, err := readHostPressure()
		if err != nil {
			return err
		}
		last = pressure
		if maintenancePressureOK(pressure, maxLoad, minMemoryMB) {
			if attempt > 1 {
				log.Printf(
					"patchctl maintenance pressure recovered phase=%s load_1m=%.2f available_memory_mb=%d",
					phase,
					pressure.Load1M,
					pressure.AvailableMemoryMB,
				)
			}
			return nil
		}

		log.Printf(
			"patchctl maintenance pressure wait phase=%s attempt=%d/%d load_1m=%.2f max_load_1m=%.2f available_memory_mb=%d min_available_memory_mb=%d",
			phase,
			attempt,
			attempts,
			pressure.Load1M,
			maxLoad,
			pressure.AvailableMemoryMB,
			minMemoryMB,
		)
		timer := time.NewTimer(time.Duration(sleepSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf(
		"maintenance pressure stayed high before %s: load_1m=%.2f available_memory_mb=%d",
		phase,
		last.Load1M,
		last.AvailableMemoryMB,
	)
}

func maintenancePressureOK(pressure hostPressure, maxLoad float64, minMemoryMB int) bool {
	if maxLoad > 0 && pressure.Load1M > maxLoad {
		return false
	}
	if minMemoryMB > 0 && pressure.AvailableMemoryMB > 0 && pressure.AvailableMemoryMB < minMemoryMB {
		return false
	}
	return true
}

func readHostPressure() (hostPressure, error) {
	load, err := readLoad1M("/proc/loadavg")
	if err != nil {
		return hostPressure{}, err
	}
	memoryMB, err := readAvailableMemoryMB("/proc/meminfo")
	if err != nil {
		return hostPressure{}, err
	}
	return hostPressure{Load1M: load, AvailableMemoryMB: memoryMB}, nil
}

func readLoad1M(path string) (float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0, fmt.Errorf("%s did not contain load averages", path)
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s load average: %w", path, err)
	}
	return value, nil
}

func readAvailableMemoryMB(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "MemAvailable:" {
			continue
		}
		valueKB, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, fmt.Errorf("parse %s MemAvailable: %w", path, err)
		}
		return valueKB / 1024, nil
	}
	return 0, fmt.Errorf("%s did not contain MemAvailable", path)
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
