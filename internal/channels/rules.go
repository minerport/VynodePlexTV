package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type Field struct {
	Name      string   `json:"name"`
	Label     string   `json:"label"`
	Kind      string   `json:"kind"`
	Operators []string `json:"operators"`
}

var fields = []Field{
	{"library", "Library", "reference", nil}, {"server", "Server", "reference", nil}, {"media_type", "Media type", "enum", nil}, {"title", "Title", "string", nil}, {"show", "Show", "reference", nil}, {"season", "Season", "reference", nil},
	{"genre", "Genre", "reference", nil}, {"year", "Year", "number", nil}, {"actor", "Actor", "reference", nil}, {"director", "Director", "reference", nil}, {"writer", "Writer", "reference", nil}, {"studio", "Studio", "string", nil}, {"collection", "Collection", "reference", nil}, {"label", "Label", "reference", nil}, {"content_rating", "Content rating", "enum", nil}, {"rating", "Rating", "number", nil}, {"audience_rating", "Audience rating", "number", nil}, {"added_at", "Date added", "date", nil}, {"originally_available_at", "Release date", "date", nil}, {"duration", "Duration (minutes)", "number", nil}, {"availability", "Available", "boolean", nil},
}

func Fields() []Field {
	out := make([]Field, len(fields))
	copy(out, fields)
	for i := range out {
		switch out[i].Kind {
		case "string":
			out[i].Operators = []string{"equals", "not_equals", "contains", "not_contains", "starts_with", "ends_with"}
		case "number":
			out[i].Operators = []string{"equals", "not_equals", "greater_than", "greater_than_or_equal", "less_than", "less_than_or_equal", "between"}
		case "date":
			out[i].Operators = []string{"before", "after", "between", "within_last_days"}
		case "boolean":
			out[i].Operators = []string{"is_true", "is_false"}
		default:
			out[i].Operators = []string{"is", "is_not", "in", "not_in"}
		}
	}
	return out
}

type Predicate struct {
	SQL  string
	Args []any
}
type RuleCompiler struct {
	DB  queryer
	Now func() time.Time
}

func (c RuleCompiler) Compile(ctx context.Context, r Rule) (Predicate, error) {
	if r.Version != 1 {
		return Predicate{}, invalid("rule.version", "Unsupported rule version; version 1 is required")
	}
	n := 0
	clock := time.Now().UTC()
	if c.Now != nil {
		clock = c.Now().UTC()
	}
	return c.node(ctx, r.Root, "rule.root", 1, &n, clock)
}
func (c RuleCompiler) node(ctx context.Context, n Node, path string, depth int, count *int, clock time.Time) (Predicate, error) {
	*count++
	if depth > 8 || *count > 64 {
		return Predicate{}, invalid(path, "Rules allow at most 8 levels and 64 nodes")
	}
	bad := func(message string) (Predicate, error) { return Predicate{}, invalid(path, message) }
	if n.Type == "group" {
		if n.Field != "" || len(n.Value) > 0 {
			return bad("Groups cannot contain condition values")
		}
		if n.Operator != "AND" && n.Operator != "OR" && n.Operator != "NOT" {
			return bad("Choose AND, OR or NOT")
		}
		if len(n.Children) == 0 || len(n.Children) > 64 || n.Operator == "NOT" && len(n.Children) != 1 {
			return bad("Groups require children; NOT requires exactly one child")
		}
		p := Predicate{}
		parts := []string{}
		for i, child := range n.Children {
			v, e := c.node(ctx, child, pathChild(path, i), depth+1, count, clock)
			if e != nil {
				return p, e
			}
			parts = append(parts, v.SQL)
			p.Args = append(p.Args, v.Args...)
		}
		if n.Operator == "NOT" {
			p.SQL = "NOT (" + parts[0] + ")"
		} else {
			p.SQL = "(" + strings.Join(parts, " "+n.Operator+" ") + ")"
		}
		return p, nil
	}
	if n.Type != "condition" || len(n.Children) > 0 {
		return bad("Expected a condition or group")
	}
	var field *Field
	for _, f := range Fields() {
		if f.Name == n.Field {
			x := f
			field = &x
			break
		}
	}
	if field == nil {
		return bad("Unknown rule field")
	}
	allowed := false
	for _, op := range field.Operators {
		if op == n.Operator {
			allowed = true
		}
	}
	if !allowed {
		return bad("Operator is not valid for this field")
	}
	var value any
	if len(n.Value) > 0 {
		if json.Unmarshal(n.Value, &value) != nil {
			return bad("Invalid condition value")
		}
	}
	columns := map[string]string{"title": "m.title", "studio": "m.studio", "year": "m.year", "rating": "m.rating", "audience_rating": "m.audience_rating", "duration": "m.duration_ms/60000.0", "availability": "m.available", "added_at": "m.added_at", "originally_available_at": "CAST(strftime('%s',NULLIF(m.originally_available_at,'')) AS INTEGER)", "content_rating": "m.content_rating", "media_type": "m.media_type"}
	column := columns[n.Field]
	p := Predicate{}
	negative := false
	switch field.Kind {
	case "boolean":
		if value != nil {
			return bad("Boolean operators do not take a value")
		}
		p.SQL = "m.available=1"
		negative = n.Operator == "is_false"
	case "string":
		text, ok := value.(string)
		if !ok || len(text) > 500 {
			return bad("Enter a string of at most 500 characters")
		}
		negative = n.Operator == "not_equals" || n.Operator == "not_contains"
		if n.Operator == "equals" || n.Operator == "not_equals" {
			p.SQL = "LOWER(" + column + ")=LOWER(?)"
			p.Args = []any{text}
		} else {
			escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(text)
			switch n.Operator {
			case "contains", "not_contains":
				escaped = "%" + escaped + "%"
			case "starts_with":
				escaped += "%"
			case "ends_with":
				escaped = "%" + escaped
			}
			p.SQL = column + " LIKE ? ESCAPE '\\'"
			p.Args = []any{escaped}
		}
	case "number":
		nums, e := numbers(value, n.Operator == "between")
		if e != nil {
			return bad(e.Error())
		}
		ops := map[string]string{"equals": "=", "not_equals": "=", "greater_than": ">", "greater_than_or_equal": ">=", "less_than": "<", "less_than_or_equal": "<="}
		negative = n.Operator == "not_equals"
		if n.Operator == "between" {
			p.SQL = column + " BETWEEN ? AND ?"
			p.Args = []any{nums[0], nums[1]}
		} else {
			p.SQL = column + ops[n.Operator] + "?"
			p.Args = []any{nums[0]}
		}
	case "date":
		if n.Operator == "within_last_days" {
			days, ok := value.(float64)
			if !ok || days < 1 || days > 36500 || math.Trunc(days) != days {
				return bad("Enter a whole number of days between 1 and 36500")
			}
			p.SQL = column + " BETWEEN ? AND ?"
			p.Args = []any{clock.Add(-time.Duration(days) * 24 * time.Hour).Unix(), clock.Unix()}
		} else {
			values := []any{value}
			if n.Operator == "between" {
				var ok bool
				values, ok = value.([]any)
				if !ok || len(values) != 2 {
					return bad("Choose two dates")
				}
			}
			dates := []int64{}
			for _, v := range values {
				text, ok := v.(string)
				if !ok {
					return bad("Enter an ISO date or RFC3339 timestamp")
				}
				d, e := time.Parse(time.RFC3339, text)
				if e != nil {
					d, e = time.Parse("2006-01-02", text)
				}
				if e != nil {
					return bad("Enter an ISO date or RFC3339 timestamp")
				}
				dates = append(dates, d.Unix())
			}
			if len(dates) == 2 {
				if dates[0] > dates[1] {
					return bad("Range start must not exceed its end")
				}
				p.SQL = column + " BETWEEN ? AND ?"
				p.Args = []any{dates[0], dates[1]}
			} else {
				op := "<"
				if n.Operator == "after" {
					op = ">"
				}
				p.SQL = column + op + "?"
				p.Args = []any{dates[0]}
			}
		}
	default:
		values := []any{value}
		if n.Operator == "in" || n.Operator == "not_in" {
			var ok bool
			values, ok = value.([]any)
			if !ok || len(values) == 0 || len(values) > 64 {
				return bad("Select between 1 and 64 values")
			}
		}
		ids := []string{}
		for _, v := range values {
			id, ok := v.(string)
			if !ok || id == "" || len(id) > 300 {
				return bad("Choose a valid value")
			}
			if e := c.reference(ctx, n.Field, id); e != nil {
				if _, ok := e.(*Error); ok {
					return bad(e.Error())
				}
				return p, e
			}
			ids = append(ids, id)
		}
		negative = n.Operator == "is_not" || n.Operator == "not_in"
		p = relationship(n.Field, ids)
	}
	p.SQL = "COALESCE((" + p.SQL + "),0)"
	if negative {
		p.SQL = "NOT (" + p.SQL + ")"
	}
	return p, nil
}
func numbers(value any, pair bool) ([]float64, error) {
	values := []any{value}
	if pair {
		var ok bool
		values, ok = value.([]any)
		if !ok || len(values) != 2 {
			return nil, fmt.Errorf("Enter two numbers")
		}
	}
	out := []float64{}
	for _, v := range values {
		n, ok := v.(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 1e12 {
			return nil, fmt.Errorf("Enter a finite number")
		}
		out = append(out, n)
	}
	if pair && out[0] > out[1] {
		return nil, fmt.Errorf("Range start must not exceed its end")
	}
	return out, nil
}
func (c RuleCompiler) reference(ctx context.Context, field, id string) error {
	if field == "media_type" {
		if !IsPlayable(id) {
			return invalid("value", "Choose movie or episode")
		}
		return nil
	}
	if field == "content_rating" {
		return nil
	}
	var query string
	args := []any{id}
	switch field {
	case "server":
		query = "SELECT EXISTS(SELECT 1 FROM plex_servers WHERE id=?)"
	case "library":
		query = "SELECT EXISTS(SELECT 1 FROM plex_libraries WHERE id=?)"
	case "show", "season":
		query = "SELECT EXISTS(SELECT 1 FROM plex_media_items WHERE id=? AND media_type=?)"
		args = append(args, field)
	case "collection":
		entity, lib, ok := collectionParts(id)
		if !ok {
			return invalid("value", "Choose a collection with library context")
		}
		query = "SELECT EXISTS(SELECT 1 FROM metadata_entities e JOIN media_item_entities r ON r.entity_id=e.id JOIN plex_media_items m ON m.id=r.media_item_id WHERE e.id=? AND e.kind='collection' AND m.plex_library_id=?)"
		args = []any{entity, lib}
	default:
		kind := field
		if field == "actor" || field == "director" || field == "writer" {
			kind = "person"
		}
		query = "SELECT EXISTS(SELECT 1 FROM metadata_entities e JOIN media_item_entities r ON r.entity_id=e.id WHERE e.id=? AND e.kind=? AND r.relationship=?)"
		args = append(args, kind, field)
	}
	var exists bool
	if e := c.DB.QueryRowContext(ctx, query, args...).Scan(&exists); e != nil {
		return e
	}
	if !exists {
		return invalid("value", "Referenced metadata no longer exists")
	}
	return nil
}
func collectionParts(id string) (string, string, bool) {
	parts := strings.Split(id, "@")
	if len(parts) != 2 || parts[1] == "" {
		return "", "", false
	}
	n, e := strconv.ParseInt(parts[0], 10, 64)
	return parts[0], parts[1], e == nil && n > 0
}

// Episode relationships include their parent show's metadata. This permits genres and
// cast defined only at show level to match episodes without duplicating Plex records.
const relationOwner = "r.media_item_id IN (m.id, (SELECT parent.id FROM plex_media_items parent WHERE m.media_type='episode' AND parent.plex_server_id=m.plex_server_id AND parent.plex_rating_key=m.grandparent_rating_key AND parent.media_type='show'))"

func relationship(field string, ids []string) Predicate {
	p := Predicate{}
	for _, id := range ids {
		p.Args = append(p.Args, id)
	}
	direct := map[string]string{"server": "m.plex_server_id", "library": "m.plex_library_id", "media_type": "m.media_type", "content_rating": "m.content_rating"}
	if column := direct[field]; column != "" {
		p.SQL = column + " IN (" + placeholders(len(ids)) + ")"
		return p
	}
	if field == "show" || field == "season" {
		key := "m.grandparent_rating_key"
		if field == "season" {
			key = "m.parent_rating_key"
		}
		p.SQL = "EXISTS (SELECT 1 FROM plex_media_items parent WHERE parent.plex_server_id=m.plex_server_id AND parent.plex_rating_key=" + key + " AND parent.id IN (" + placeholders(len(ids)) + "))"
		return p
	}
	if field == "collection" {
		p.Args = nil
		parts := []string{}
		for _, id := range ids {
			entity, lib, _ := collectionParts(id)
			parts = append(parts, "(m.plex_library_id=? AND EXISTS (SELECT 1 FROM media_item_entities r WHERE r.relationship='collection' AND r.entity_id=? AND "+relationOwner+"))")
			p.Args = append(p.Args, lib, entity)
		}
		p.SQL = "(" + strings.Join(parts, " OR ") + ")"
		return p
	}
	p.SQL = "EXISTS (SELECT 1 FROM media_item_entities r WHERE r.relationship=? AND r.entity_id IN (" + placeholders(len(ids)) + ") AND " + relationOwner + ")"
	p.Args = append([]any{field}, p.Args...)
	return p
}
