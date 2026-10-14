// Command v1diff compares the old API's rankings with the /v1 ones that
// replace them (docs/REBUILD.md M4): for each pair it fetches the top N of
// both and reports how many entries they share and how many sit at the same
// position, and with -v lists the entries only one side has.
//
// A pair that depends on classification (domains, forms, shelves) or on a
// full day of growth is flagged: its differences are expected while the new
// labels or growth history differ from the old ones, and want explaining
// rather than fixing.
//
//	v1diff -old https://api.starrank.dev -new http://localhost:8080 [-n 100] [-v] [-only apps]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// side is one endpoint of a pair: a path with its query, the page size it
// accepts, and the field that identifies a row.
type side struct {
	path    string
	perPage int
	id      string
	paged   bool
}

type pair struct {
	name     string
	old, new side
	// tree marks a ranking that depends on the labels; growth one that
	// depends on a full day of growth.
	tree, growth bool
}

func repos(q string) (side, side) {
	return side{"/trending?source=github&" + q, 50, "externalId", true}, side{"/v1/repos?" + q, 50, "fullName", true}
}

func pairs() []pair {
	var ps []pair
	add := func(name string, old, new side, tree, growth bool) {
		ps = append(ps, pair{name, old, new, tree, growth})
	}
	for _, s := range []struct {
		sort   string
		growth bool
	}{{"stars", false}, {"forks", false}, {"daily", true}, {"weekly", true}, {"monthly", true}} {
		o, n := repos("sort=" + s.sort)
		add("repos "+s.sort, o, n, false, s.growth)
	}
	o, n := repos("sort=stars&language=Go")
	add("repos stars Go", o, n, false, false)
	o, _ = repos("sort=stars&category=ai")
	_, n = repos("sort=stars&domain=ai")
	add("repos stars ai", o, n, true, false)
	o, _ = repos("sort=weekly&category=agents")
	_, n = repos("sort=weekly&domain=agents")
	add("repos weekly agents", o, n, true, true)

	for _, w := range []string{"daily", "weekly", "monthly"} {
		add("hot "+w, side{"/trending/hot?since=" + w, 0, "externalId", false}, side{"/v1/hot?since=" + w + "&limit=50", 0, "fullName", false}, false, false)
	}
	for _, b := range []struct {
		board  string
		growth bool
	}{{"merit", false}, {"rising", true}} {
		add("developers "+b.board, side{"/developers?board=" + b.board + "&domain=all&limit=100", 0, "login", false},
			side{"/v1/developers?board=" + b.board + "&domain=all&limit=100", 0, "login", false}, false, b.growth)
	}
	for _, s := range []struct {
		sort   string
		growth bool
	}{{"popular", false}, {"hot", true}} {
		add("topics "+s.sort+" ai", side{"/topics?sort=" + s.sort + "&domain=ai&limit=60", 0, "topic", false},
			side{"/v1/topics?sort=" + s.sort + "&domain=ai&limit=60", 0, "topic", false}, true, s.growth)
	}
	for _, s := range []struct {
		sort   string
		growth bool
	}{{"popular", false}, {"hot", true}} {
		add("ecosystem all "+s.sort, side{"/ecosystem?pillar=all&sort=" + s.sort, 100, "externalId", true},
			side{"/v1/ecosystem?pillar=all&sort=" + s.sort, 100, "externalId", true}, true, s.growth)
	}
	add("languages", side{"/stats/languages?limit=30", 0, "language", false}, side{"/v1/stats/languages?limit=30", 0, "language", false}, false, false)
	add("heat", side{"/category/heat", 0, "path", false}, side{"/v1/heat", 0, "path", false}, true, true)
	for _, s := range []struct {
		sort   string
		growth bool
	}{{"likes", false}, {"downloads", false}, {"hot", true}} {
		add("models "+s.sort, side{"/models?sort=" + s.sort, 100, "externalId", true},
			side{"/v1/models?sort=" + s.sort, 100, "externalId", true}, false, s.growth)
	}
	for _, s := range []struct {
		q      string
		growth bool
	}{{"sort=popular", false}, {"sort=popular&os=macos", false}, {"sort=new", false}, {"sort=hot", true}} {
		add("apps "+s.q, side{"/apps?" + s.q, 100, "externalId", true}, side{"/v1/apps?" + s.q, 100, "externalId", true}, true, s.growth)
	}
	add("alternatives", side{"/alternatives", 0, "slug", false}, side{"/v1/alternatives", 0, "slug", false}, true, false)
	return ps
}

var client = &http.Client{Timeout: 3 * time.Minute}

// fetch reads the ids of the first n rows of s.
func fetch(base string, s side, n int) ([]string, error) {
	var ids []string
	for page := 1; len(ids) < n; page++ {
		u := base + s.path
		if s.paged {
			sep := "?"
			if strings.Contains(u, "?") {
				sep = "&"
			}
			u += sep + url.Values{"limit": {fmt.Sprint(s.perPage)}, "page": {fmt.Sprint(page)}}.Encode()
		}
		resp, err := client.Get(u)
		if err != nil {
			return nil, err
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", u, resp.Status)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", u, err)
		}
		for _, row := range body.Data {
			ids = append(ids, fmt.Sprint(row[s.id]))
		}
		if !s.paged || len(body.Data) < s.perPage {
			break
		}
	}
	return ids[:min(n, len(ids))], nil
}

func main() {
	oldBase := flag.String("old", "https://api.starrank.dev", "old API base URL")
	newBase := flag.String("new", "http://localhost:8080", "new API base URL")
	n := flag.Int("n", 100, "how deep to compare")
	verbose := flag.Bool("v", false, "list the entries only one side has")
	only := flag.String("only", "", "compare only the pairs whose name contains this")
	flag.Parse()

	tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "pair\told\tnew\tshared\tsame pos\tdepends on")
	var details []string
	failed := false
	for _, p := range pairs() {
		if *only != "" && !strings.Contains(p.name, *only) {
			continue
		}
		o, err := fetch(*oldBase, p.old, *n)
		if err == nil {
			var nw []string
			nw, err = fetch(*newBase, p.new, *n)
			if err == nil {
				shared, same, oldOnly, newOnly := compare(o, nw)
				var deps []string
				if p.tree {
					deps = append(deps, "labels")
				}
				if p.growth {
					deps = append(deps, "growth")
				}
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%s\n", p.name, len(o), len(nw), shared, same, strings.Join(deps, ","))
				if *verbose && (len(oldOnly) > 0 || len(newOnly) > 0) {
					details = append(details, fmt.Sprintf("== %s\n  old only: %s\n  new only: %s", p.name,
						strings.Join(oldOnly, " "), strings.Join(newOnly, " ")))
				}
				continue
			}
		}
		failed = true
		fmt.Fprintf(tw, "%s\terror: %v\n", p.name, err)
	}
	tw.Flush()
	for _, d := range details {
		fmt.Println(d)
	}
	if failed {
		os.Exit(1)
	}
}

// compare counts the ids both lists have and those at the same position, and
// lists, with their 1-based positions, the ones only one list has.
func compare(old, new []string) (shared, same int, oldOnly, newOnly []string) {
	inOld, inNew := map[string]bool{}, map[string]bool{}
	for _, id := range old {
		inOld[id] = true
	}
	for i, id := range new {
		inNew[id] = true
		if i < len(old) && old[i] == id {
			same++
		}
	}
	for i, id := range old {
		if inNew[id] {
			shared++
		} else {
			oldOnly = append(oldOnly, fmt.Sprintf("%d:%s", i+1, id))
		}
	}
	for i, id := range new {
		if !inOld[id] {
			newOnly = append(newOnly, fmt.Sprintf("%d:%s", i+1, id))
		}
	}
	return shared, same, oldOnly, newOnly
}
