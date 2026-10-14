package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func TestAppEndpoints(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	ps := []item.Alternative{{Name: "Photoshop", Slug: "photoshop"}}
	apps := []boards.App{
		{ExternalID: "a/shot", Stars: 100, Growth: 50, Kind: "app", Shelf: "system/screenshot", Group: "system", Platforms: []string{"macos"}, Domains: []string{"apps"}, Listed: true},
		{ExternalID: "a/files", Stars: 900, Growth: 5, Kind: "cli", Shelf: "system/files", Group: "system", Platforms: []string{"linux"}, Listed: true},
		{ExternalID: "a/term", Stars: 500, Kind: "app", Shelf: "devtools/terminal", Group: "dev", Platforms: []string{"macos"}, Listed: true},
		{ExternalID: "a/gimp", Stars: 300, Kind: "app", Alternatives: ps, Platforms: []string{}},
	}
	rows := make([]store.AppRow, len(apps))
	for i, a := range apps {
		a.At = at
		rows[i] = a
	}
	if _, err := st.PutApps(ctx, rows, at); err != nil {
		t.Fatal(err)
	}
	r := router(st)
	var out struct {
		Data  []boards.App `json:"data"`
		Total int          `json:"total"`
	}
	ids := func() string {
		s := ""
		for _, a := range out.Data {
			s += a.ExternalID + " "
		}
		return s
	}
	for url, want := range map[string]string{
		"/v1/apps":                             "a/shot a/files a/term ",
		"/v1/apps?sort=popular":                "a/files a/term a/shot ",
		"/v1/apps?shelf=system":                "a/shot a/files ",
		"/v1/apps?shelf=dev":                   "a/term ",
		"/v1/apps?shelf=system/files":          "a/files ",
		"/v1/apps?os=macos&kind=app":           "a/shot a/term ",
		"/v1/apps?domain=apps":                 "a/shot ",
		"/v1/apps?sort=popular&limit=1&page=2": "a/term ",
	} {
		out.Data = nil
		if w := get(t, r, url, nil, &out); w.Code != http.StatusOK || ids() != want {
			t.Errorf("%s = %d %q; want %q", url, w.Code, ids(), want)
		}
	}
	out.Data = nil
	if w := get(t, r, "/v1/apps?limit=100&page=3", nil, &out); w.Code != http.StatusOK || len(out.Data) != 0 || out.Total != 3 {
		t.Errorf("past the store's depth = %d %+v; want an empty page", w.Code, out)
	}
	for _, url := range []string{"/v1/apps?os=beos", "/v1/apps?kind=gui", "/v1/apps?shelf=a/b/c", "/v1/apps?sort=stars"} {
		if w := get(t, r, url, nil, nil); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d; want 400", url, w.Code)
		}
	}

	var alt struct {
		Data []boards.App `json:"data"`
		Name string       `json:"name"`
	}
	if w := get(t, r, "/v1/alternatives/photoshop", nil, &alt); w.Code != http.StatusOK || alt.Name != "Photoshop" ||
		len(alt.Data) != 1 || alt.Data[0].ExternalID != "a/gimp" {
		t.Fatalf("by alternative = %d %+v; unlisted apps still replace products", w.Code, alt)
	}
	if w := get(t, r, "/v1/alternatives/nothing", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown product = %d", w.Code)
	}

	must(t, st.PutBoard(ctx, boards.BestOfKey("system/files"), []boards.App{apps[1]}, at))
	must(t, st.PutBoard(ctx, boards.AltTargetsKey, []boards.AltTarget{{Name: "Photoshop", Slug: "photoshop", Count: 1}}, at))
	var best struct {
		Data  []boards.App `json:"data"`
		Shelf string       `json:"shelf"`
	}
	if w := get(t, r, "/v1/apps/best/system/files", nil, &best); w.Code != http.StatusOK || best.Shelf != "system/files" || len(best.Data) != 1 {
		t.Fatalf("best = %d %+v", w.Code, best)
	}
	if w := get(t, r, "/v1/apps/best/system/nope", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown shelf = %d", w.Code)
	}
	var targets struct {
		Data []boards.AltTarget `json:"data"`
	}
	if w := get(t, r, "/v1/alternatives", nil, &targets); w.Code != http.StatusOK || len(targets.Data) != 1 {
		t.Fatalf("targets = %d %+v", w.Code, targets)
	}
}
