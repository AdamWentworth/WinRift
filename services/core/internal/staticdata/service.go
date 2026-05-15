package staticdata

import (
	"context"
	"fmt"
	"sync"
)

type RiotClient interface {
	DataDragonVersions(ctx context.Context) ([]string, error)
	DataDragonJSON(ctx context.Context, version, path string) (any, error)
}

type Service struct {
	riot          RiotClient
	mu            sync.Mutex
	latestVersion string
	cache         map[string]any
}

func NewService(riot RiotClient) *Service {
	return &Service{riot: riot, cache: map[string]any{}}
}

func (s *Service) Get(ctx context.Context, kind, patch string) (map[string]any, error) {
	fileByKind := map[string]string{
		"champions":       "champion.json",
		"items":           "item.json",
		"runes":           "runesReforged.json",
		"summoner-spells": "summoner.json",
	}
	file, ok := fileByKind[kind]
	if !ok {
		return nil, fmt.Errorf("unknown static data kind")
	}
	version := patch
	if version == "" {
		var err error
		version, err = s.LatestVersion(ctx)
		if err != nil {
			return nil, err
		}
	}
	key := version + ":" + kind
	s.mu.Lock()
	if data, ok := s.cache[key]; ok {
		s.mu.Unlock()
		return map[string]any{"version": version, "data": data}, nil
	}
	s.mu.Unlock()
	data, err := s.riot.DataDragonJSON(ctx, version, file)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[key] = data
	s.mu.Unlock()
	return map[string]any{"version": version, "data": data}, nil
}

func (s *Service) LatestVersion(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.latestVersion != "" {
		defer s.mu.Unlock()
		return s.latestVersion, nil
	}
	s.mu.Unlock()
	versions, err := s.riot.DataDragonVersions(ctx)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no Data Dragon versions returned")
	}
	s.mu.Lock()
	s.latestVersion = versions[0]
	s.mu.Unlock()
	return versions[0], nil
}
