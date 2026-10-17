package channels

import (
	"context"
	"encoding/json"
)

func (s *Service) RecentKinds(ctx context.Context) ([]string, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT DISTINCT json_extract(config_json,'$.recent.kind') FROM channel_sources WHERE json_extract(config_json,'$.recent.kind') IS NOT NULL")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Recent sources are system-assisted RULE templates, with a canonical rule enforced server-side.
func RecentRule(v RecentConfig) Rule {
	kind := "movie"
	if v.Kind == "TV" {
		kind = "episode"
	}
	node := func(field, op string, value any) Node {
		b, _ := json.Marshal(value)
		return Node{Type: "condition", Field: field, Operator: op, Value: b}
	}
	return Rule{Version: 1, Root: Node{Type: "group", Operator: "AND", Children: []Node{node("media_type", "is", kind), {Type: "condition", Field: "availability", Operator: "is_true"}, node("added_at", "within_last_days", v.Days)}}}
}
func (s *Service) CreateRecent(ctx context.Context, kind, actor string) (Channel, error) {
	if kind != "MOVIES" && kind != "TV" {
		return Channel{}, invalid("kind", "Choose MOVIES or TV")
	}
	number, e := s.NextNumber(ctx)
	if e != nil {
		return Channel{}, e
	}
	name := "Recently Added Movies"
	group := "movies"
	if kind == "TV" {
		name = "Recently Added TV"
		group = "tv"
	}
	r := RecentConfig{Kind: kind, Days: 30}
	rule := RecentRule(r)
	return s.Save(ctx, "", Definition{Number: number, Name: name, ShortName: name, Description: "Fresh additions to your collection, on their own frequency.", Enabled: true, Visibility: "VISIBLE", GroupID: &group, Logo: Appearance{Source: "AUTO"}, Background: Appearance{Source: "AUTO"}, Source: Source{Type: "RULE", Rule: &rule, Recent: &r}}, actor)
}
