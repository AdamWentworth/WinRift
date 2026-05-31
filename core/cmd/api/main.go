package main

import (
	"log"
	"net/http"

	"winrift/core/internal/api"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
	"winrift/core/internal/staticdata"
)

func main() {
	cfg := config.Load()
	riot.ClearAuthFailureMarker(cfg)
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
