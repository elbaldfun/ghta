package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/elbaldfun/ghta/internal/boards"
)

// The ecosystem board and /v1/repos show one repository's day's gain the
// same, whenever the board was computed: the board below holds the night
// before's numbers, as it does while the fetch runs.
func TestEcosystemGrowthMatchesRepos(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	artex := ghRepo("baarai/artex", "Python", 13891, 11016)
	artex.Growth.D1 = f64(1643)
	agent := ghRepo("acme/agent", "Go", 500, 40)
	agent.Growth.D1 = f64(1700)
	quiet := ghRepo("acme/quiet", "Go", 90, 0) // no gain measured today
	gone := ghRepo("acme/gone", "Go", 10, 0)
	gone.Stale = true
	seed(t, st, artex, agent, quiet, gone)
	yesterday := []boards.EcoItem{
		{ExternalID: "baarai/artex", Stars: 12248, Growth: 2776, IsMcp: true},
		{ExternalID: "acme/agent", Stars: 480, Growth: 20, IsAgent: true},
		{ExternalID: "acme/quiet", Stars: 90, Growth: 5, IsAgent: true},
		{ExternalID: "acme/gone", Stars: 10, Growth: 1, IsAgent: true},
		{ExternalID: "acme/deleted", Stars: 1, Growth: 0, IsAgent: true},
	}
	at := time.Date(2026, 11, 21, 7, 26, 0, 0, time.UTC)
	must(t, st.PutBoard(ctx, boards.EcosystemKey(boards.PillarAll, boards.EcoHot), yesterday, at))
	must(t, st.PutBoard(ctx, boards.EcosystemKey(boards.PillarAll, boards.EcoPopular), yesterday, at))
	r := router(st)

	var repos listPage
	if w := get(t, r, "/v1/repos?limit=50", nil, &repos); w.Code != http.StatusOK {
		t.Fatalf("repos = %d", w.Code)
	}
	d1 := map[string]int{}
	for _, c := range repos.Data {
		if c.Growth.D1 != nil {
			d1[c.FullName] = int(*c.Growth.D1)
		}
	}

	for sort, order := range map[string]string{
		boards.EcoHot:     "acme/agent,baarai/artex,acme/quiet",
		boards.EcoPopular: "baarai/artex,acme/agent,acme/quiet",
	} {
		var eco struct {
			Data  []boards.EcoItem `json:"data"`
			Total int              `json:"total"`
		}
		if w := get(t, r, "/v1/ecosystem?sort="+sort, nil, &eco); w.Code != http.StatusOK {
			t.Fatalf("ecosystem %s = %d", sort, w.Code)
		}
		var ids []string
		for _, e := range eco.Data {
			ids = append(ids, e.ExternalID)
			if e.Growth != d1[e.ExternalID] {
				t.Errorf("%s %s: ecosystem growth %d, repos d1 %d", sort, e.ExternalID, e.Growth, d1[e.ExternalID])
			}
		}
		if got := strings.Join(ids, ","); got != order || eco.Total != 3 {
			t.Errorf("%s order = %s (%d); want %s", sort, got, eco.Total, order)
		}
		if a := eco.Data[0]; sort == boards.EcoPopular && a.Stars != 13891 {
			t.Errorf("stars = %s", fmt.Sprint(a.Stars))
		}
	}
}
