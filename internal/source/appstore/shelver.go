package appstore

import (
	"strconv"

	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// NewShelver maps a listing to a shelf by taxonomy/store-genres.yaml: its
// primary genre, then keywords in its name or description.
func NewShelver(sg *taxonomy.StoreGenres) Shelver {
	return func(a App) string {
		return sg.Shelf(strconv.Itoa(a.PrimaryGenreID), a.TrackName, a.Description)
	}
}
