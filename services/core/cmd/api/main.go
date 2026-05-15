package main

import (
	"log"
	"net/http"

	"winrift/services/core/internal/api"
	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
	"winrift/services/core/internal/staticdata"
)

func main() {
	cfg := config.Load()
	riot.ClearAuthFailureMarker(cfg)
	riot.StartAuthFailureMonitor(cfg, "winrift api")
	riotClient := riot.NewClient(cfg)
	repo, err := clickhouse.NewRepository(cfg)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	staticService := staticdata.NewService(riotClient)
	server := api.NewServer(cfg, riotClient, repo, staticService)

	log.Printf("winrift api listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
