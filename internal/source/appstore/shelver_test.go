package appstore

import (
	"path/filepath"
	"testing"

	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// The genre table must say something about every genre the job charts, a
// shelf or a reason for none, so a genre added to Genres is a decision.
func TestShelverCoversGenres(t *testing.T) {
	root := filepath.Join("..", "..", "..", "taxonomy")
	tree, err := taxonomy.LoadTree(filepath.Join(root, "taxonomy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sg, err := taxonomy.LoadStoreGenres(filepath.Join(root, "store-genres.yaml"), tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range Genres {
		if !sg.Has(g.ID) {
			t.Errorf("genre %s (%s) missing from store-genres.yaml", g.ID, g.Slug)
		}
	}
	shelve := NewShelver(sg)
	for _, c := range []struct {
		a    App
		want string
	}{
		{App{TrackName: "Robinhood: Investing for All", PrimaryGenreID: 6015}, "finance/investing"},
		{App{TrackName: "Chase Mobile", PrimaryGenreID: 6015}, "finance/payments"},
		{App{TrackName: "Clash Royale", PrimaryGenreID: 6014}, ""},
	} {
		if got := shelve(c.a); got != c.want {
			t.Errorf("%s → %q, want %q", c.a.TrackName, got, c.want)
		}
	}
}
