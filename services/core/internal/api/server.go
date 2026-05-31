package api

import (
	"log"
	"net/http"

	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/collector"
	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
	"winrift/services/core/internal/staticdata"
	"winrift/services/core/internal/winconditions"
)

type Server struct {
	cfg           config.Config
	riot          *riot.Client
	repo          *clickhouse.Repository
	static        *staticdata.Service
	collector     collector.Collector
	winConds      winconditions.Analyzer
	responseCache *responseCache
}

func NewServer(cfg config.Config, riotClient *riot.Client, repo *clickhouse.Repository, staticService *staticdata.Service) Server {
	catalog, err := winconditions.LoadCatalog()
	if err != nil {
		log.Printf("win condition catalog load failed err=%v", err)
	}
	return Server{cfg: cfg, riot: riotClient, repo: repo, static: staticService, collector: collector.New(riotClient, repo), winConds: winconditions.NewAnalyzer(catalog), responseCache: newResponseCache()}
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/account/resolve", s.resolveAccount)
	mux.HandleFunc("GET /api/account/alias", s.accountAlias)
	mux.HandleFunc("GET /api/account/aliases", s.accountAliases)
	mux.HandleFunc("GET /api/summoner/profile", s.summonerProfile)
	mux.HandleFunc("GET /api/summoners/leaderboard", s.summonerLeaderboard)
	mux.HandleFunc("GET /api/live-game", s.liveGame)
	mux.HandleFunc("GET /api/analytics/builds", s.analyticsBuilds)
	mux.HandleFunc("GET /api/analytics/build-advice", s.analyticsBuildAdvice)
	mux.HandleFunc("GET /api/analytics/champion-page", s.analyticsChampionPage)
	mux.HandleFunc("GET /api/analytics/champion-guides", s.analyticsChampionGuideIndex)
	mux.HandleFunc("GET /api/analytics/champion-guide", s.analyticsChampionGuide)
	mux.HandleFunc("GET /api/analytics/item-slots", s.analyticsItemSlots)
	mux.HandleFunc("POST /api/analytics/item-slots/batch", s.analyticsItemSlotsBatch)
	mux.HandleFunc("GET /api/analytics/champion-roles", s.analyticsChampionRoles)
	mux.HandleFunc("GET /api/analytics/patches", s.analyticsPatches)
	mux.HandleFunc("POST /api/analytics/win-conditions", s.analyticsWinConditions)
	mux.HandleFunc("GET /api/analytics/win-conditions/diagnostics", s.analyticsWinConditionDiagnostics)
	mux.HandleFunc("GET /api/static/{kind}", s.staticData)
	mux.HandleFunc("POST /api/dev/collector/seed", s.seedCollector)
	mux.HandleFunc("POST /api/dev/analytics/item-slots/refresh", s.refreshItemSlotAnalytics)
	mux.HandleFunc("POST /api/dev/analytics/champion-guides/refresh", s.refreshChampionGuideAnalytics)
	mux.HandleFunc("POST /api/dev/analytics/summoner-profiles/refresh", s.refreshSummonerProfileAnalytics)
	return s.cors(mux)
}

func (s Server) health(w http.ResponseWriter, r *http.Request) {
	riotStatus := "ok"
	if riot.AuthFailureMarkerExists(s.cfg) {
		riotStatus = "auth_failed"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "riotApi": riotStatus})
}
