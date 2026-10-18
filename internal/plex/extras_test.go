package plex

import (
	"encoding/xml"
	"testing"
)

func TestTrailerMetadataParsing(t *testing.T) {
	var c xmlContainer
	e := xml.Unmarshal([]byte(`<MediaContainer><Video ratingKey="1" type="movie"><Extras><Video ratingKey="2" type="clip" extraType="1" title="Trailer" duration="123000"/><Video ratingKey="3" type="clip" extraType="2" title="Interview" duration="99000"/></Extras></Video></MediaContainer>`), &c)
	if e != nil || len(c.Videos) != 1 || len(c.Videos[0].Extras.Videos) != 2 {
		t.Fatal(c, e)
	}
	x := c.Videos[0].Extras.Videos[0]
	if x.ExtraType != 1 || x.RatingKey != "2" || *x.Duration != 123000 {
		t.Fatal(x)
	}
}
