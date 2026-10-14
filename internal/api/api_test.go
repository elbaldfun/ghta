package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database("ghta_api_test")
	if err := db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Drop(ctx); _ = client.Disconnect(ctx) })
	st := store.New(db)
	if err := st.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}

func router(st *store.Store) *gin.Engine { return routerWith(New(st, nil)) }

func routerWith(a *API) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	a.Register(r)
	return r
}

// get performs a request and decodes a JSON body into out, when given.
func get(t *testing.T, r http.Handler, url string, header map[string]string, out any) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if out != nil && w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
	return w
}

func TestBoardEndpoints(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	devs := []boards.Developer{{Login: "a", Growth: 3}, {Login: "b", Growth: 2}, {Login: "c", Growth: 1}}
	must(t, st.PutBoard(ctx, boards.DevelopersKey(boards.DevRising, "ai"), devs, at))
	must(t, st.PutBoard(ctx, boards.HeatKey, []boards.HeatCell{{Path: "ai", Name: "人工智能"}}, at))
	must(t, st.PutBoard(ctx, boards.StalenessKey, boards.Staleness{Total: 3,
		Examples: []boards.StaleRepo{{ExternalID: "o/1"}, {ExternalID: "o/2"}}}, at))
	r := router(st)

	var page struct {
		Data       []boards.Developer `json:"data"`
		Total      int                `json:"total"`
		Page       int                `json:"page"`
		ComputedAt time.Time          `json:"computedAt"`
	}
	w := get(t, r, "/v1/developers?domain=ai&limit=2&page=2", nil, &page)
	if w.Code != http.StatusOK || page.Total != 3 || len(page.Data) != 1 || page.Data[0].Login != "c" || !page.ComputedAt.Equal(at) {
		t.Fatalf("page 2 = %d %+v", w.Code, page)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Fatalf("cache-control = %q", cc)
	}
	// Same bytes, same tag: the client's copy is still good.
	etag := w.Header().Get("ETag")
	if w := get(t, r, "/v1/developers?domain=ai&limit=2&page=2", map[string]string{"If-None-Match": etag}, nil); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("revalidation = %d %q", w.Code, w.Body.String())
	}
	if w := get(t, r, "/v1/developers?domain=ai&limit=2&page=1", map[string]string{"If-None-Match": etag}, nil); w.Code != http.StatusOK {
		t.Fatalf("another page answered %d to the first page's tag", w.Code)
	}

	for url, code := range map[string]int{
		"/v1/developers?domain=nope":      http.StatusNotFound, // no such board
		"/v1/developers?board=best":       http.StatusBadRequest,
		"/v1/developers?limit=101":        http.StatusBadRequest,
		"/v1/topics":                      http.StatusNotFound, // not computed yet
		"/v1/ecosystem?pillar=x":          http.StatusBadRequest,
		"/v1/stats/staleness?examples=99": http.StatusBadRequest,
	} {
		if w := get(t, r, url, nil, nil); w.Code != code {
			t.Errorf("%s = %d; want %d", url, w.Code, code)
		}
	}

	var heat struct {
		Data []boards.HeatCell `json:"data"`
	}
	if w := get(t, r, "/v1/heat", nil, &heat); w.Code != http.StatusOK || len(heat.Data) != 1 || heat.Data[0].Name != "人工智能" {
		t.Fatalf("heat = %d %+v", w.Code, heat)
	}
	var stale struct {
		Data boards.Staleness `json:"data"`
	}
	get(t, r, "/v1/stats/staleness?examples=1", nil, &stale)
	if stale.Data.Total != 3 || len(stale.Data.Examples) != 1 {
		t.Fatalf("staleness = %+v", stale.Data)
	}
	get(t, r, "/v1/stats/staleness", nil, &stale)
	if len(stale.Data.Examples) != 0 {
		t.Fatalf("examples default to none: %+v", stale.Data.Examples)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestModelsEndpoint(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	must(t, st.PutBoard(ctx, boards.ModelsKey("vision", boards.ModelLikes), []boards.Model{{ExternalID: "a/b", Likes: 3}}, at))
	r := router(st)
	var out struct {
		Data  []boards.Model `json:"data"`
		Total int            `json:"total"`
	}
	if w := get(t, r, "/v1/models?task=vision&sort=likes", nil, &out); w.Code != http.StatusOK || out.Total != 1 || out.Data[0].Likes != 3 {
		t.Fatalf("models = %d %+v", w.Code, out)
	}
	if w := get(t, r, "/v1/models?sort=size", nil, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad sort = %d", w.Code)
	}
	if w := get(t, r, "/v1/models", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing board = %d", w.Code)
	}
}

func TestHotEndpoint(t *testing.T) {
	st := testStore(t)
	at := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	must(t, st.PutBoard(context.Background(), boards.HotKey("daily"),
		[]boards.HotRepo{{FullName: "a/b", StarsGained: 9, Tracked: true}, {FullName: "c/d"}}, at))
	r := router(st)
	var out struct {
		Data  []boards.HotRepo `json:"data"`
		Since string           `json:"since"`
	}
	if w := get(t, r, "/v1/hot?limit=1", nil, &out); w.Code != http.StatusOK || out.Since != "daily" ||
		len(out.Data) != 1 || out.Data[0].StarsGained != 9 {
		t.Fatalf("hot = %d %+v", w.Code, out)
	}
	if w := get(t, r, "/v1/hot?since=yearly", nil, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad window = %d", w.Code)
	}
	if w := get(t, r, "/v1/hot?since=weekly", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing board = %d", w.Code)
	}
}
