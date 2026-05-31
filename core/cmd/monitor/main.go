package main

import (
	"context"
	"errors"
	"log"

	"winrift/core/internal/config"
	"winrift/core/internal/monitor"
)

func main() {
	cfg := config.Load()
	service := monitor.NewService(cfg, monitor.NewEmailNotifier(cfg))
	if err := service.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
