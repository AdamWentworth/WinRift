package clickhouse

import "strings"

type roleAnalyticsScope struct {
	selectExpr string
	whereSQL   string
	args       []any
}

// analyticsRoleScope intentionally merges solo lanes for build advice, where
// matchup-specific items usually transfer between top and mid.
func analyticsRoleScope(role string) roleAnalyticsScope {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "TOP":
		return roleAnalyticsScope{selectExpr: "'TOP'", whereSQL: "role IN ('TOP', 'MIDDLE')"}
	case "MIDDLE":
		return roleAnalyticsScope{selectExpr: "'MIDDLE'", whereSQL: "role IN ('TOP', 'MIDDLE')"}
	case "JUNGLE":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"JUNGLE"}}
	case "BOTTOM":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"BOTTOM"}}
	case "UTILITY":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"UTILITY"}}
	default:
		return roleAnalyticsScope{selectExpr: "'ALL'"}
	}
}

func analyticsOpponentBucketExpr(filters map[string]string) string {
	if strings.TrimSpace(filters["opponent_champion_id"]) != "" {
		return "opponent_champion_id"
	}
	return "toUInt16(0)"
}

// strictAnalyticsRoleScope keeps champion strength and guide rankings in their
// selected role so mid-lane picks do not leak into top-lane tier lists.
func strictAnalyticsRoleScope(role string) roleAnalyticsScope {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "TOP":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"TOP"}}
	case "MIDDLE":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"MIDDLE"}}
	case "JUNGLE":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"JUNGLE"}}
	case "BOTTOM":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"BOTTOM"}}
	case "UTILITY":
		return roleAnalyticsScope{selectExpr: "role", whereSQL: "role = ?", args: []any{"UTILITY"}}
	default:
		return roleAnalyticsScope{selectExpr: "'ALL'"}
	}
}
