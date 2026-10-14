package api

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

const maxAgeBadge = 3600

func (a *API) registerBadge(r gin.IRouter) {
	r.GET("/badge/:owner/:file", a.badge)
}

// GET /v1/badge/:owner/:name.svg?theme=light|dark&lang=en|zh — a README
// badge with the repository's best ranking: its best domain, else its
// language, else GitHub as a whole. Only tracked, live repositories get one;
// the rest get a grey "not ranked" badge with a 404.
func (a *API) badge(c *gin.Context) {
	theme, err := oneOf(c, "theme", "light", "dark")
	if err != nil {
		a.fail(c, err)
		return
	}
	lang, err := oneOf(c, "lang", "en", "zh")
	if err != nil {
		a.fail(c, err)
		return
	}
	name, ok := strings.CutSuffix(c.Param("file"), ".svg")
	if !ok {
		a.fail(c, errNotFound)
		return
	}
	notRanked := map[string]string{"en": "not ranked", "zh": "未上榜"}[lang]
	it, err := a.st.Item(c.Request.Context(), item.SourceGitHub, c.Param("owner")+"/"+name)
	if err != nil || it.Stale || it.Ranks == nil || it.Ranks.Overall == 0 {
		if err != nil && !errors.Is(err, store.ErrNoItem) {
			a.fail(c, err)
			return
		}
		a.cachedBytes(c, maxAgeBadge, http.StatusNotFound, svgType, badgeSVG("StarRank", notRanked, theme, false))
		return
	}
	a.cachedBytes(c, maxAgeBadge, http.StatusOK, svgType, badgeSVG("StarRank", a.bestRank(c, &it, lang), theme, true))
}

const svgType = "image/svg+xml; charset=utf-8"

// bestRank phrases the item's best ranking: the domain where it ranks
// highest (the item's first domain on a tie), else its language, else all of
// GitHub.
func (a *API) bestRank(c *gin.Context, it *item.Item, lang string) string {
	best, at := "", 0
	for _, p := range it.Labels.Domain {
		if n := it.Ranks.Domain[p]; n > 0 && (at == 0 || n < at) {
			best, at = p, n
		}
	}
	in := map[string]string{"en": "in", "zh": ""}[lang]
	if best != "" {
		return fmt.Sprintf("#%d %s %s", at, in, a.domainName(c, best, lang))
	}
	if it.Ranks.Language > 0 && it.Language != "" {
		return fmt.Sprintf("#%d %s %s", it.Ranks.Language, in, it.Language)
	}
	return fmt.Sprintf("#%d %s GitHub", it.Ranks.Overall, map[string]string{"en": "on", "zh": ""}[lang])
}

// domainName names a domain leaf from the categories board, or by its path
// when the board does not have it.
func (a *API) domainName(c *gin.Context, path, lang string) string {
	var tree []boards.CategoryNode
	if _, err := a.st.Board(c.Request.Context(), boards.CategoriesKey, &tree); err != nil {
		return path
	}
	var find func([]boards.CategoryNode) string
	find = func(nodes []boards.CategoryNode) string {
		for _, n := range nodes {
			if n.Path == path {
				if lang == "en" && n.NameEn != "" {
					return n.NameEn
				}
				return n.Name
			}
			if strings.HasPrefix(path, n.Path+"/") {
				return find(n.Children)
			}
		}
		return ""
	}
	if name := find(tree); name != "" {
		return name
	}
	return path
}

// badgeSVG draws a flat two-part badge. Text widths are estimated and pinned
// with textLength, so every renderer lays it out the same.
func badgeSVG(label, value, theme string, ranked bool) []byte {
	value = strings.Join(strings.Fields(value), " ")
	lw, vw := textWidth(label)+12, textWidth(value)+12
	left, right, fg := "#555", "#1e8e3e", "#fff"
	if theme == "dark" {
		left, right, fg = "#30363d", "#2ea043", "#f0f6fc"
	}
	if !ranked {
		right = "#9f9f9f"
		if theme == "dark" {
			right = "#484f58"
		}
	}
	title := html.EscapeString(label + ": " + value)
	l, v := html.EscapeString(label), html.EscapeString(value)
	w := lw + vw
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s">`+
		`<title>%s</title>`+
		`<clipPath id="r"><rect width="%d" height="20" rx="3"/></clipPath>`+
		`<g clip-path="url(#r)"><rect width="%d" height="20" fill="%s"/><rect x="%d" width="%d" height="20" fill="%s"/></g>`+
		`<g fill="%s" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">`+
		`<text x="%d" y="14" textLength="%d">%s</text><text x="%d" y="14" textLength="%d">%s</text></g></svg>`,
		w, title, title, w, lw, left, lw, vw, right, fg,
		lw/2, lw-12, l, lw+vw/2, vw-12, v))
}

// textWidth estimates the width of s in 11px Verdana: about 7px a Latin
// character and 11px a CJK one.
func textWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r >= 0x2E80:
			w += 11
		case r == ' ' || r == 'i' || r == 'l' || r == '.' || r == ',' || r == '1':
			w += 4
		default:
			w += 7
		}
	}
	return w
}
