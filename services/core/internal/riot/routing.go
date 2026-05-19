package riot

import (
	"fmt"
	"net/url"
	"strings"
)

var platformAliases = map[string]string{
	"BR": "BR1", "EUN": "EUN1", "EUNE": "EUN1", "EUW": "EUW1",
	"JP": "JP1", "KR": "KR", "LAN": "LA1", "LAS": "LA2",
	"NA": "NA1", "OCE": "OC1", "TR": "TR1", "RU": "RU",
	"PH": "PH2", "SG": "SG2", "TH": "TH2", "TW": "TW2", "VN": "VN2",
}

var regionByPlatform = map[string]string{
	"BR1": "AMERICAS", "LA1": "AMERICAS", "LA2": "AMERICAS", "NA1": "AMERICAS",
	"OC1": "SEA", "JP1": "ASIA", "KR": "ASIA",
	"EUN1": "EUROPE", "EUW1": "EUROPE", "RU": "EUROPE", "TR1": "EUROPE",
	"PH2": "SEA", "SG2": "SEA", "TH2": "SEA", "TW2": "SEA", "VN2": "SEA",
}

func NormalizePlatform(platform string) string {
	value := strings.ToUpper(strings.TrimSpace(platform))
	if value == "" {
		value = "NA1"
	}
	if alias, ok := platformAliases[value]; ok {
		return alias
	}
	return value
}

func RegionForPlatform(platform string) (string, error) {
	normalized := NormalizePlatform(platform)
	region, ok := regionByPlatform[normalized]
	if !ok {
		return "", fmt.Errorf("unsupported platform %q", platform)
	}
	return region, nil
}

func AccountRegionForPlatform(platform string) (string, error) {
	region, err := RegionForPlatform(platform)
	if err != nil {
		return "", err
	}
	if region == "SEA" {
		return "ASIA", nil
	}
	return region, nil
}

func ParseRiotID(value string) (string, string, error) {
	parts := strings.Split(value, "#")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("riot id must be gameName#tagLine")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func escapePath(value string) string {
	return url.PathEscape(value)
}
