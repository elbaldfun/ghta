// Package api is the /v1 HTTP interface of the rebuilt store
// (docs/REDESIGN.md 5.5). It reads items, item_blobs, boards and apps only; every
// response is a declared DTO, never a stored document.
//
// Lists and boards are cacheable: they carry Cache-Control and an ETag, and a
// matching If-None-Match gets a 304, so a CDN in front can absorb the
// crawlers.
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/store"
)

// Cache lifetimes, in seconds. Boards change once a night; lists move with
// the same nightly fetch but are cheap to refresh more often.
const (
	maxAgeList  = 300
	maxAgeBoard = 3600
)

type API struct {
	st       *store.Store
	log      *slog.Logger
	backfill Backfill
}

func New(st *store.Store, log *slog.Logger) *API {
	if log == nil {
		log = slog.Default()
	}
	return &API{st: st, log: log}
}

// Register mounts the /v1 routes.
func (a *API) Register(r gin.IRouter) {
	v1 := r.Group("/v1")
	a.registerBoards(v1)
	a.registerRepos(v1)
	a.registerDetail(v1)
	a.registerApps(v1)
	a.registerBadge(v1)
}

// errBadInput marks a request the client got wrong.
type errBadInput string

func (e errBadInput) Error() string { return string(e) }

var errNotFound = errors.New("not found")

// fail writes an error response: 400 for bad input, 404 for nothing there,
// and 500 (logged, not echoed) for anything else.
func (a *API) fail(c *gin.Context, err error) {
	var bad errBadInput
	switch {
	case errors.As(err, &bad):
		c.JSON(http.StatusBadRequest, gin.H{"error": bad.Error()})
	case errors.Is(err, errNotFound), errors.Is(err, store.ErrNoBoard):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		a.log.Error("api: internal error", "path", c.FullPath(), "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// cached writes body as JSON with a public cache lifetime and an ETag of the
// bytes, answering 304 when the client already holds them.
func (a *API) cached(c *gin.Context, maxAge int, body any) {
	b, err := json.Marshal(body)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cachedBytes(c, maxAge, http.StatusOK, "application/json; charset=utf-8", b)
}

// cachedBytes writes b with a public cache lifetime and an ETag of the bytes.
func (a *API) cachedBytes(c *gin.Context, maxAge, status int, contentType string, b []byte) {
	sum := sha256.Sum256(b)
	etag := `"` + hex.EncodeToString(sum[:12]) + `"`
	c.Header("Cache-Control", "public, max-age="+strconv.Itoa(maxAge))
	c.Header("ETag", etag)
	if match := c.GetHeader("If-None-Match"); match != "" && match == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(status, contentType, b)
}

// intParam reads an optional integer query parameter within [lo, hi].
func intParam(c *gin.Context, name string, def, lo, hi int) (int, error) {
	v := c.Query(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, errBadInput(name + " must be " + strconv.Itoa(lo) + ".." + strconv.Itoa(hi))
	}
	return n, nil
}

// oneOf reads an optional enum query parameter; the first choice is the
// default.
func oneOf(c *gin.Context, name string, choices ...string) (string, error) {
	v := c.Query(name)
	if v == "" {
		return choices[0], nil
	}
	for _, ch := range choices {
		if v == ch {
			return v, nil
		}
	}
	msg := name + " must be one of"
	for i, ch := range choices {
		if i > 0 {
			msg += ","
		}
		msg += " " + ch
	}
	return "", errBadInput(msg)
}

// page cuts one page out of rows.
func page[T any](rows []T, limit, page int) []T {
	start := (page - 1) * limit
	if start >= len(rows) {
		return []T{}
	}
	return rows[start:min(start+limit, len(rows))]
}
