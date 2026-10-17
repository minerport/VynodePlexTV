package channels

import (
	"afterglow.local/tv/internal/database"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	db, e := database.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO users VALUES('owner','owner','unused','OWNER',1)`)
	exec(`INSERT INTO plex_servers(id,name,base_url,machine_identifier,token_encrypted,token_key_version,created_at,updated_at) VALUES('server','Fixture','http://unused.invalid','fixture',x'00',1,1,1)`)
	exec(`INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,selected,created_at,updated_at) VALUES('library','server','1','Fixture library','show',1,1,1)`)
	for _, m := range []struct {
		id, kind, title, parent, grand string
		year, season, index, available int
	}{{"a", "movie", "Alpha Action", "", "", 2015, 0, 0, 1}, {"b", "movie", "Beta Comedy", "", "", 1995, 0, 0, 1}, {"d", "movie", "Dark Horror", "", "", 2000, 0, 0, 0}, {"z", "show", "Show Z", "", "", 2020, 0, 0, 1}, {"s1", "season", "Season 1", "z", "", 2020, 0, 1, 1}, {"s2", "season", "Season 2", "z", "", 2020, 0, 2, 1}, {"s10", "season", "Season 10", "z", "", 2020, 0, 10, 1}, {"s0", "season", "Specials", "z", "", 2020, 0, 0, 1}, {"c", "episode", "Second Episode", "s2", "z", 2020, 2, 2, 1}, {"e", "episode", "Tenth Episode", "s2", "z", 2020, 2, 10, 1}, {"f", "episode", "Special", "s0", "z", 2020, 0, 1, 1}} {
		exec(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,parent_rating_key,grandparent_rating_key,year,parent_index_number,index_number,available,rating,audience_rating,duration_ms,added_at,originally_available_at,studio,content_rating,created_at,updated_at) VALUES(?,'server','library',?,?,?,?,?,?,?,?,?,8,9,3600000,?,'2020-01-01','Studio A','PG',1,1)`, m.id, m.id, m.kind, m.title, m.parent, m.grand, m.year, m.season, m.index, m.available, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC).Unix())
	}
	for _, e := range []struct {
		id         int
		kind, name string
	}{{1, "genre", "Action"}, {2, "genre", "Comedy"}, {3, "genre", "Horror"}, {4, "genre", "Sci-Fi"}, {5, "person", "Actor X"}, {6, "person", "Actor Y"}, {7, "person", "Director X"}, {8, "person", "Writer X"}, {9, "label", "Family"}, {10, "collection", "Universe"}} {
		exec("INSERT INTO metadata_entities VALUES(?,'server',?,?,?)", e.id, e.kind, e.name, e.name)
	}
	for _, r := range []struct {
		item string
		id   int
		role string
	}{{"a", 1, "genre"}, {"b", 2, "genre"}, {"d", 3, "genre"}, {"z", 4, "genre"}, {"a", 5, "actor"}, {"b", 6, "actor"}, {"a", 7, "director"}, {"z", 8, "writer"}, {"a", 9, "label"}, {"a", 10, "collection"}, {"z", 10, "collection"}} {
		exec("INSERT INTO media_item_entities VALUES(?,?,?,'')", r.item, r.id, r.role)
	}
	return New(db, dir)
}
func condition(field, op string, value any) Node {
	var raw json.RawMessage
	if value != nil {
		raw, _ = json.Marshal(value)
	}
	return Node{Type: "condition", Field: field, Operator: op, Value: raw}
}
func group(op string, children ...Node) Node {
	return Node{Type: "group", Operator: op, Children: children}
}
func ruleSource(n Node) Source { return Source{Type: "RULE", Rule: &Rule{Version: 1, Root: n}} }
func base(source Source) Definition {
	return Definition{Number: "101", Name: "Fixture channel", Enabled: true, Visibility: "VISIBLE", Logo: Appearance{Source: "AUTO"}, Background: Appearance{Source: "AUTO"}, Source: source}
}
func ids(p Preview) string {
	out := []string{}
	for _, m := range p.Items {
		out = append(out, m.ID)
	}
	return strings.Join(out, ",")
}
func TestRuleGoldenMembership(t *testing.T) {
	s := fixture(t)
	cases := []struct {
		name string
		node Node
		want string
	}{
		{"genre", condition("genre", "is", "1"), "a"}, {"decade", condition("year", "between", []int{1990, 1999}), "b"}, {"OR", group("OR", condition("genre", "is", "1"), condition("genre", "is", "2")), "a,b"},
		{"NOT", group("AND", condition("media_type", "is", "movie"), group("NOT", condition("genre", "is", "3"))), "a,b"},
		{"nested", group("AND", group("OR", condition("genre", "is", "1"), condition("genre", "is", "2")), group("NOT", condition("year", "less_than", 2000))), "a"},
		{"episode inherited genre", condition("genre", "is", "4"), "c,f,e"}, {"show", condition("show", "is", "z"), "c,f,e"}, {"season", condition("season", "is", "s2"), "c,e"},
		{"actor", condition("actor", "is", "5"), "a"}, {"director", condition("director", "is", "7"), "a"}, {"writer inherited", condition("writer", "is", "8"), "c,f,e"}, {"label", condition("label", "is", "9"), "a"}, {"collection", condition("collection", "is", "10@library"), "a,c,f,e"},
		{"library", condition("library", "is", "library"), "a,b,d,c,f,e"}, {"server", condition("server", "is", "server"), "a,b,d,c,f,e"}, {"availability", condition("availability", "is_false", nil), "d"},
		{"equals", condition("title", "equals", "alpha action"), "a"}, {"not_equals", condition("title", "not_equals", "Alpha Action"), "b,d,c,f,e"}, {"contains", condition("title", "contains", "Episode"), "c,e"}, {"not_contains", condition("title", "not_contains", "Episode"), "a,b,d,f"}, {"starts", condition("title", "starts_with", "Beta"), "b"}, {"ends", condition("title", "ends_with", "Action"), "a"},
		{"number equals", condition("year", "equals", 1995), "b"}, {"number not_equals", condition("year", "not_equals", 2020), "a,b,d"}, {"gt", condition("year", "greater_than", 2015), "c,f,e"}, {"gte", condition("year", "greater_than_or_equal", 2015), "a,c,f,e"}, {"lt", condition("year", "less_than", 2000), "b"}, {"lte", condition("year", "less_than_or_equal", 2000), "b,d"},
		{"rating", condition("rating", "between", []int{7, 9}), "a,b,d,c,f,e"}, {"audience", condition("audience_rating", "greater_than", 8), "a,b,d,c,f,e"}, {"duration minutes", condition("duration", "equals", 60), "a,b,d,c,f,e"},
		{"date before", condition("originally_available_at", "before", "2021-01-01"), "a,b,d,c,f,e"}, {"date after", condition("added_at", "after", "2026-10-01"), "a,b,d,c,f,e"}, {"date range", condition("added_at", "between", []string{"2026-10-01", "2026-10-15"}), "a,b,d,c,f,e"},
		{"enum in", condition("content_rating", "in", []string{"PG", "G"}), "a,b,d,c,f,e"}, {"enum notin", condition("media_type", "not_in", []string{"movie"}), "c,f,e"}, {"relationship isnot", condition("genre", "is_not", "4"), "a,b,d"}, {"studio", condition("studio", "equals", "Studio A"), "a,b,d,c,f,e"}, {"zero", condition("title", "contains", "no matching name"), ""}, {"injection", condition("title", "contains", "%' OR 1=1 --"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, e := s.Preview(context.Background(), PreviewRequest{Source: ruleSource(tc.node), IncludeUnavailable: true, Limit: 100})
			if e != nil {
				t.Fatal(e)
			}
			if ids(p) != tc.want {
				t.Fatalf("got %s want %s", ids(p), tc.want)
			}
		})
	}
	c := RuleCompiler{DB: s.db, Now: func() time.Time { return time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC) }}
	p, e := c.Compile(context.Background(), Rule{1, condition("added_at", "within_last_days", 30)})
	if e != nil {
		t.Fatal(e)
	}
	var count int
	if e = s.db.QueryRow("SELECT count(*) FROM plex_media_items m WHERE "+playableSQL+" AND "+p.SQL, p.Args...).Scan(&count); e != nil || count != 6 {
		t.Fatal("relative date outcome", count, e)
	}
}
func TestRuleValidationLimitsAndNullSemantics(t *testing.T) {
	s := fixture(t)
	deep := condition("year", "equals", 2020)
	for i := 0; i < 9; i++ {
		deep = group("AND", deep)
	}
	cases := []Node{condition("unknown", "is", "x"), condition("year", "contains", 2020), condition("year", "equals", "2020"), group("AND"), group("NOT", condition("year", "equals", 1), condition("year", "equals", 2)), deep, condition("show", "is", "missing"), condition("genre", "in", []string{}), condition("added_at", "before", "not-date"), condition("duration", "between", []int{10, 1}), condition("media_type", "is", "show"), condition("collection", "is", "Universe")}
	for i, n := range cases {
		if _, e := (RuleCompiler{DB: s.db}).Compile(context.Background(), Rule{1, n}); e == nil {
			t.Fatalf("invalid case %d accepted", i)
		}
	}
	if _, e := (RuleCompiler{DB: s.db}).Compile(context.Background(), Rule{2, condition("year", "equals", 2000)}); e == nil {
		t.Fatal("unknown version")
	}
	nodes := []Node{}
	for i := 0; i < 65; i++ {
		nodes = append(nodes, condition("year", "equals", 2000))
	}
	if _, e := (RuleCompiler{DB: s.db}).Compile(context.Background(), Rule{1, group("AND", nodes...)}); e == nil {
		t.Fatal("node cap")
	}
	s.db.Exec("UPDATE plex_media_items SET year=NULL WHERE id='a'")
	p, e := s.Preview(context.Background(), PreviewRequest{Source: ruleSource(group("NOT", condition("year", "equals", 2000))), IncludeUnavailable: true})
	if e != nil || !strings.Contains(ids(p), "a") {
		t.Fatal("NOT must complement unknown positive value", e)
	}
}
func TestChannelLifecycleStaticAvailabilityAndAudit(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	d := base(Source{Type: "STATIC", Static: &StaticConfig{[]string{"b", "d", "a"}}})
	c, e := s.Save(ctx, "", d, "owner")
	if e != nil {
		t.Fatal(e)
	}
	if c.Count != 2 || len(c.Source.Static.MediaIDs) != 3 {
		t.Fatal("availability retention")
	}
	p, e := s.ResolveChannelMedia(ctx, c.ID, true, 0, 100)
	if e != nil || ids(p) != "b,d,a" {
		t.Fatal(ids(p), e)
	}
	p, e = s.ResolveChannelMedia(ctx, c.ID, false, 1, 1)
	if e != nil || ids(p) != "a" || p.Count != 2 {
		t.Fatal("pagination", p, e)
	}
	if _, e = s.Save(ctx, "", d, "owner"); e == nil {
		t.Fatal("duplicate number")
	}
	d.Number = "MOV1"
	d.Name = "Updated"
	c, e = s.Save(ctx, c.ID, d, "owner")
	if e != nil || c.Name != "Updated" {
		t.Fatal(e)
	}
	copy, e := s.Duplicate(ctx, c.ID, "owner")
	if e != nil || copy.ID == c.ID || copy.Number == c.Number || copy.Name != "Updated Copy" {
		t.Fatal("duplicate", e)
	}
	if e = s.Enable(ctx, c.ID, false, "owner"); e != nil {
		t.Fatal(e)
	}
	c, e = s.Get(ctx, c.ID)
	if e != nil || c.Enabled {
		t.Fatal("disable")
	}
	s.db.Exec("UPDATE plex_media_items SET available=0 WHERE id='a'")
	c, e = s.Get(ctx, c.ID)
	if e != nil || c.Count != 1 || len(c.Source.Static.MediaIDs) != 3 {
		t.Fatal("later unavailability lost membership")
	}
	if e = s.Delete(ctx, c.ID, "owner"); e != nil {
		t.Fatal(e)
	}
	var inventory, audits int
	s.db.QueryRow("SELECT count(*) FROM plex_media_items").Scan(&inventory)
	s.db.QueryRow("SELECT count(*) FROM audit_log WHERE user_id='owner' AND target_id<>'' AND summary<>''").Scan(&audits)
	if inventory != 11 || audits < 5 {
		t.Fatal(inventory, audits)
	}
	if _, e = s.Save(ctx, "", base(Source{Type: "STATIC", Static: &StaticConfig{[]string{"z"}}}), "owner"); e == nil {
		t.Fatal("container accepted")
	}
	if _, e = s.Save(ctx, "", base(Source{Type: "STATIC", Static: &StaticConfig{[]string{"a", "a"}}}), "owner"); e == nil {
		t.Fatal("duplicate static accepted")
	}
}
func TestShowCollectionGroupsAndOfflineResolution(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	source := Source{Type: "SHOW", Show: &ShowConfig{ShowID: "z", EpisodeScope: "ALL"}}
	p, e := s.Preview(ctx, PreviewRequest{Source: source})
	if e != nil || ids(p) != "c,e" {
		t.Fatal("numeric ordering", ids(p), e)
	}
	source.Show.IncludeSpecials = true
	p, e = s.Preview(ctx, PreviewRequest{Source: source})
	if e != nil || ids(p) != "f,c,e" {
		t.Fatal("specials", ids(p), e)
	}
	source.Show.EpisodeScope = "SELECTED_SEASONS"
	source.Show.SeasonIDs = []string{"s1"}
	p, e = s.Preview(ctx, PreviewRequest{Source: source})
	if e != nil || p.Count != 0 || len(p.Warnings) == 0 {
		t.Fatal("empty channel", e)
	}
	c, e := s.Save(ctx, "", base(source), "owner")
	if e != nil {
		t.Fatal(e)
	}
	g, e := s.SaveGroup(ctx, "", Group{Name: "Custom", Slug: "custom"}, "owner")
	if e != nil {
		t.Fatal(e)
	}
	d := c.Definition
	d.GroupID = &g.ID
	c, e = s.Save(ctx, c.ID, d, "owner")
	if e != nil {
		t.Fatal(e)
	}
	g.Name = "Renamed"
	g.SortOrder = 1
	if _, e = s.SaveGroup(ctx, g.ID, g, "owner"); e != nil {
		t.Fatal(e)
	}
	if e = s.DeleteGroup(ctx, g.ID, nil, false, "owner"); e == nil {
		t.Fatal("nonempty group silently removed")
	}
	if e = s.DeleteGroup(ctx, g.ID, nil, true, "owner"); e != nil {
		t.Fatal(e)
	}
	c, e = s.Get(ctx, c.ID)
	if e != nil || c.GroupID != nil {
		t.Fatal("ungroup")
	}
	s.db.Exec("UPDATE plex_servers SET status='OFFLINE',base_url='http://127.0.0.1:1'")
	collection := Source{Type: "COLLECTION", Collection: &CollectionConfig{"10@library"}}
	p, e = s.Preview(ctx, PreviewRequest{Source: collection})
	if e != nil || p.Count != 4 {
		t.Fatal("offline collection", e)
	}
	for _, field := range []string{"genre", "actor", "director", "writer", "collection", "label", "show", "season", "library", "server", "studio", "content_rating"} {
		options, e := s.Metadata(ctx, field, "", "", 0, 10)
		if e != nil || len(options.Items) == 0 {
			t.Fatalf("metadata %s: %v", field, e)
		}
	}
	s.db.Exec("UPDATE plex_libraries SET selected=0")
	p, e = s.Preview(ctx, PreviewRequest{Source: collection})
	if e != nil || p.Count != 0 {
		t.Fatal("deselected library included")
	}
}
func TestArtworkValidationAndPersistence(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	var b bytes.Buffer
	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 16, 16)))
	u, e := s.Upload(ctx, b.Bytes(), "owner")
	if e != nil {
		t.Fatal(e)
	}
	if u.Width != 16 {
		t.Fatal("dimensions")
	}
	d := base(Source{Type: "STATIC", Static: &StaticConfig{[]string{"a"}}})
	d.Logo = Appearance{Source: "UPLOAD", UploadID: &u.ID}
	c, e := s.Save(ctx, "", d, "owner")
	if e != nil {
		t.Fatal(e)
	}
	copy, e := s.Duplicate(ctx, c.ID, "owner")
	if e != nil {
		t.Fatal(e)
	}
	s.Delete(ctx, c.ID, "owner")
	a, e := s.ChannelArtwork(ctx, copy.ID, "logo")
	if e != nil || !bytes.Equal(a.Data, b.Bytes()) {
		t.Fatal("shared upload deleted", e)
	}
	for _, bad := range [][]byte{[]byte(`<svg onload="alert(1)"/>`), b.Bytes()[:30], make([]byte, MaxUpload+1)} {
		if _, e = s.Upload(ctx, bad, "owner"); e == nil {
			t.Fatal("bad upload accepted")
		}
	}
	if _, e = s.UploadArtwork(ctx, "../../secrets/plex-key-v1"); e == nil {
		t.Fatal("path traversal")
	}
	if strings.Contains(string(Fallback("<script>", "&").Data), "<script>") {
		t.Fatal("fallback not escaped")
	}
}
