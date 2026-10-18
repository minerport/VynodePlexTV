package personalization

import (
	"afterglow.local/tv/internal/channels"
	"context"
	"encoding/json"
	"testing"
)

func TestTemplatesUseExistingRuleCompiler(t *testing.T) {
	s := personalFixture(t)
	for _, v := range []TemplateInput{{Template: "genre", Selection: "1"}, {Template: "actor", Selection: "2"}, {Template: "director", Selection: "3"}, {Template: "decade", Decade: 1990}, {Template: "weekend", Selection: "1"}, {Template: "family", Selection: "1"}} {
		t.Run(v.Template, func(t *testing.T) {
			source, e := TemplateSource(v)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = (channels.RuleCompiler{DB: s.db}).Compile(context.Background(), *source.Rule); e != nil {
				b, _ := json.Marshal(source)
				t.Fatal(string(b), e)
			}
		})
	}
	if _, e := TemplateSource(TemplateInput{Template: "actor"}); e != ErrInvalid {
		t.Fatal(e)
	}
	if _, e := TemplateSource(TemplateInput{Template: "decade", Decade: 1991}); e != ErrInvalid {
		t.Fatal(e)
	}
}
func TestSeasonalTimezoneBounds(t *testing.T) {
	for _, v := range []struct {
		start, end string
		hours      int64
	}{{"2026-03-08", "2026-03-08", 23}, {"2026-11-01", "2026-11-01", 25}, {"2026-12-31", "2027-01-01", 48}} {
		a, b, e := WindowBounds(v.start, v.end, "America/New_York")
		if e != nil || (b-a)/3600000 != v.hours {
			t.Fatal(a, b, e)
		}
	}
	if _, _, e := WindowBounds("2026-12-31", "2026-01-01", "UTC"); e != ErrInvalid {
		t.Fatal(e)
	}
}

func TestShowTemplateUsesSequentialShowSource(t *testing.T) {
	source, err := TemplateSource(TemplateInput{Template: "comfort", Selection: "local-show"})
	if err != nil || source.Type != "SHOW" || source.Rule != nil || source.Show == nil || source.Show.ShowID != "local-show" || source.Show.EpisodeScope != "ALL" {
		t.Fatalf("invalid show source: %+v %v", source, err)
	}
	if _, err = TemplateSource(TemplateInput{Template: "comfort", Selection: "local-show", MediaType: "movie"}); err != ErrInvalid {
		t.Fatal("movie-only show accepted")
	}
}

func TestCustomTemplateFiltersCompile(t *testing.T) {
	s := personalFixture(t)
	v := TemplateInput{Template: "genre", Selection: "1", MediaType: "movie", YearFrom: 1990, YearTo: 1999, MinimumRating: 7, ScheduleMode: "SEQUENTIAL"}
	source, e := TemplateSource(v)
	if e != nil {
		t.Fatal(e)
	}
	if len(source.Rule.Root.Children) != 6 {
		t.Fatal(source)
	}
	if _, e = (channels.RuleCompiler{DB: s.db}).Compile(context.Background(), *source.Rule); e != nil {
		t.Fatal(e)
	}
	v.YearFrom = 2000
	if _, e = TemplateSource(v); e != ErrInvalid {
		t.Fatal("reversed range accepted")
	}
	v.YearFrom = 1990
	v.ScheduleMode = "bad"
	if _, e = TemplateSource(v); e != ErrInvalid {
		t.Fatal("invalid mode accepted")
	}
}
