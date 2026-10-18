package personalization

import (
	"afterglow.local/tv/internal/database"
	"context"
	"testing"
)

func TestPersonalOrderVisibilityAndIsolation(t *testing.T) {
	db, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO users VALUES('a','alice','unused','USER',1),('b','bob','unused','USER',1); INSERT INTO channels(id,channel_number,name,channel_type,created_at,updated_at) VALUES('one','101','One','STATIC',1,1),('two','201','Two','STATIC',1,1); INSERT INTO user_channel_favorites VALUES('a','one',1);`)
	if err != nil {
		t.Fatal(err)
	}
	s := New(db)
	ctx := context.Background()
	if err = s.Order(ctx, "a", "PERSONAL", []string{"two", "one"}); err != nil {
		t.Fatal(err)
	}
	if err = s.Hide(ctx, "a", "one", true); err != nil {
		t.Fatal(err)
	}
	a, err := s.Channels(ctx, "a")
	if err != nil || a.Mode != "PERSONAL" || a.Channels[0].ID != "two" || !a.Channels[1].Hidden || !a.Channels[1].Favorite {
		t.Fatal(a, err)
	}
	b, err := s.Channels(ctx, "b")
	if err != nil || b.Mode != "NUMBER" || b.Channels[0].ID != "one" || b.Channels[0].Hidden || b.Channels[0].Favorite {
		t.Fatal(b, err)
	}
	for _, ids := range [][]string{{"one", "one"}, {"one"}, {"one", "foreign"}} {
		if err = s.Order(ctx, "a", "PERSONAL", ids); err != ErrInvalid {
			t.Fatal("invalid permutation", err)
		}
	}
	a, err = s.Channels(ctx, "a")
	if err != nil || a.Channels[0].ID != "two" {
		t.Fatal("failed write changed order", a, err)
	}
	if err = s.Reset(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	a, err = s.Channels(ctx, "a")
	if err != nil || a.Mode != "NUMBER" || a.Channels[0].ID != "one" || a.Channels[0].Hidden || !a.Channels[0].Favorite {
		t.Fatal(a, err)
	}
	var number string
	if err = db.QueryRow("SELECT channel_number FROM channels WHERE id='one'").Scan(&number); err != nil || number != "101" {
		t.Fatal("global number changed", number, err)
	}
}
