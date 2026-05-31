package main

import (
	"context"
	"errors"
	"log"

	"winrift/services/core/internal/config"
	"winrift/services/core/internal/monitor"
)

func main() {
	cfg := config.Load()
	service := monitor.NewService(cfg, monitor.NewEmailNotifier(cfg))
	if err := service.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
