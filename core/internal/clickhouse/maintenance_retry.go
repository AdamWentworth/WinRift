package clickhouse

import (
	"context"
	"log"
	"strings"
	"time"
)

const analyticsMemoryRetryDelay = 5 * time.Second

func retryAnalyticsMemoryPressure(ctx context.Context, label string, operation func() error) error {
	return retryAnalyticsMemoryPressureWithDelay(ctx, label, analyticsMemoryRetryDelay, operation)
}

func retryAnalyticsMemoryPressureWithDelay(ctx context.Context, label string, delay time.Duration, operation func() error) error {
	for attempt := 1; ; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		if !isAnalyticsMemoryPressure(err) {
			return err
		}
		log.Printf("analytics memory pressure label=%q attempt=%d retry_in=%s error=%v", label, attempt, delay, err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func isAnalyticsMemoryPressure(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "memory limit exceeded") ||
		strings.Contains(message, "overcommittracker") ||
		strings.Contains(message, "code: 241")
}
