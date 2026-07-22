// Command eval scores the classification pipeline against the frozen golden set
// (taxonomy/golden.yaml, change 12 tasks 1.2). It replays the three domain tiers
// — rule / embedding / LLM — over the golden items and reports per-tier
// resolution, domain accuracy and multi-label precision/recall.
//
// Item metadata (topics, language, description) comes from the production API,
// so no database access is needed. Golden labels use the change-12 tree; tier
// predictions on the current tree are normalized through an alias map, and
// legacy learning/* predictions are tracked separately (that whole branch is
// removed in the new tree).
//
// The type facet is NOT evaluated yet: the type layer lands with facets.yaml
// (tasks 2.4); eval grows a type section then.
//
// Usage:
//
//	go run ./cmd/eval [-skip-embed] [-skip-llm] [-o report.json]
//
// Provider env (same names as the server): AI_PROVIDER, LMSTUDIO_BASE_URL,
// LMSTUDIO_LOCAL_MODULE_NAME, OPENAI_API_KEY, OPENAI_MODEL, EMBED_MODEL,
// EMBED_SIM_THRESHOLD.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/provider"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// aliasOldToNew maps current-tree paths to the change-12 tree used by golden.
// learning/* is intentionally absent: it maps to "no domain" (legacy bucket).
var aliasOldToNew = map[string]string{
	"web/frontend-framework": "web/frontend",
	"web/backend-framework":  "web/backend",
	"data/data-pipeline":     "data/pipeline",
	"lang/stdlib-utils":      "lang/utils",
	"data/cache":             "data/database", // merged in the new tree
}

type goldenItem struct {
	ID     string   `yaml:"id"`
	Domain []string `yaml:"domain"`
	Type   string   `yaml:"type"`
}

type goldenFile struct {
	Items []goldenItem `yaml:"items"`
}

// itemMeta is the slice of the API item the tiers need.
type itemMeta struct {
	Name        string
	Description string
	Language    string
	Topics      []string
}

// result is one item's evaluation outcome.
type result struct {
	ID            string   `json:"id"`
	Golden        []string `json:"golden"`
	GoldenType    string   `json:"goldenType"`
	Tier          string   `json:"tier"` // rule | embedding | llm | unresolved | legacy-learning
	Predicted     []string `json:"predicted"`
	PredictedType string   `json:"predictedType,omitempty"` // LLM-emitted type (measurement only)
	Hit           bool     `json:"hit"`                     // predicted ∩ golden ≠ ∅ (only meaningful when golden non-empty)
}

func main() {
	goldenPath := flag.String("golden", "taxonomy/golden.yaml", "golden set path")
	taxPath := flag.String("taxonomy", "taxonomy/taxonomy.yaml", "taxonomy path")
	mapPath := flag.String("topic-map", "taxonomy/topic-map.yaml", "topic map path")
	apiBase := flag.String("api", "https://api.starrank.dev", "production API base URL for item metadata")
	skipEmbed := flag.Bool("skip-embed", false, "skip the embedding tier")
	skipLLM := flag.Bool("skip-llm", false, "skip the LLM tier")
	sweep := flag.Bool("sweep", false, "sweep embedding thresholds over the rule-unresolved set and exit")
	batchSize := flag.Int("batch", 10, "LLM batch size")
	outPath := flag.String("o", "", "write per-item results as JSON")
	flag.Parse()

	_ = godotenv.Load()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	golden, err := loadGolden(*goldenPath)
	must(err, "load golden")
	nodes, err := taxonomy.Load(*taxPath)
	must(err, "load taxonomy")
	rules, err := taxonomy.LoadRules(*mapPath)
	must(err, "load topic map")
	leaves := leafNodes(nodes)
	fmt.Fprintf(os.Stderr, "golden=%d items, taxonomy=%d leaves\n", len(golden), len(leaves))

	// ── fetch item metadata from the prod API ──────────────────────────
	metas := map[string]itemMeta{}
	missing := 0
	for i, g := range golden {
		m, err := fetchItem(*apiBase, g.ID)
		if err != nil {
			log.Warn("fetch failed, skipping", "id", g.ID, "err", err)
			missing++
			continue
		}
		metas[g.ID] = m
		if (i+1)%50 == 0 {
			fmt.Fprintf(os.Stderr, "fetched %d/%d\n", i+1, len(golden))
		}
	}
	fmt.Fprintf(os.Stderr, "metadata: %d ok, %d missing\n", len(metas), missing)

	// ── tier 1: rule ────────────────────────────────────────────────────
	results := map[string]*result{}
	var unresolved []goldenItem
	for _, g := range golden {
		m, ok := metas[g.ID]
		if !ok {
			continue
		}
		r := &result{ID: g.ID, Golden: g.Domain, GoldenType: g.Type}
		results[g.ID] = r
		paths := rules.Classify(m.Topics, m.Language)
		norm, legacy := normalize(paths)
		switch {
		case len(norm) > 0:
			r.Tier, r.Predicted = "rule", norm
		case legacy:
			r.Tier = "legacy-learning" // rule hit learning/* only — gone in new tree
		default:
			unresolved = append(unresolved, g)
			continue
		}
	}

	// ── sweep mode: scan embedding thresholds over the rule-unresolved set ──
	cfg := envConfig()
	if *sweep {
		embedder := provider.NewEmbedder(cfg, log)
		if embedder == nil {
			must(errors.New("no embedding backend configured"), "sweep")
		}
		runSweep(ctx, embedder, leaves, metas, unresolved)
		return
	}

	// ── tier 2: embedding (current behavior: top-1 over threshold) ─────
	if !*skipEmbed {
		embedder := provider.NewEmbedder(cfg, log)
		if embedder == nil {
			fmt.Fprintln(os.Stderr, "embedding tier: no backend configured, skipping")
		} else {
			unresolved = runEmbedTier(ctx, embedder, cfg.EmbedSimThreshold, leaves, metas, unresolved, results)
		}
	}

	// ── tier 3: LLM batches ─────────────────────────────────────────────
	if !*skipLLM && len(unresolved) > 0 {
		p := provider.New(cfg, log)
		unresolved = runLLMTier(ctx, p, leaves, metas, unresolved, results, *batchSize)
	}
	for _, g := range unresolved {
		if r := results[g.ID]; r != nil && r.Tier == "" {
			r.Tier = "unresolved"
		}
	}

	report(results)
	if *outPath != "" {
		writeJSON(*outPath, results)
		fmt.Fprintf(os.Stderr, "per-item results → %s\n", *outPath)
	}
}

// normalize maps current-tree paths to new-tree paths, dropping learning/*.
// runSweep embeds the rule-unresolved items once, then reports, per threshold,
// how many the embedding tier would resolve (top-1 over threshold) and the
// hit@any accuracy among those it resolves — the data that decides whether the
// embedding tier earns its place or should just defer to the LLM.
func runSweep(ctx context.Context, embedder provider.Embedder, leaves []taxonomy.Node, metas map[string]itemMeta, pending []goldenItem) {
	// Only items that (a) rule left unresolved and (b) have a golden domain can
	// be scored for accuracy; 资料类 with empty domain are excluded.
	var scored []goldenItem
	for _, g := range pending {
		if len(g.Domain) > 0 {
			scored = append(scored, g)
		}
	}
	fmt.Fprintf(os.Stderr, "sweep: %d rule-unresolved items (%d with golden domain)\n", len(pending), len(scored))

	leafTexts := make([]string, len(leaves))
	for i, n := range leaves {
		leafTexts[i] = fmt.Sprintf("%s: %s — %s", n.Path, n.Name, n.Desc)
	}
	leafVecs, err := embedder.Embed(ctx, leafTexts)
	must(err, "embed leaves")

	texts := make([]string, len(scored))
	for i, g := range scored {
		texts[i] = itemText(metas[g.ID])
	}
	itemVecs, err := embedder.Embed(ctx, texts)
	must(err, "embed items")

	// Precompute best leaf + similarity for each item once.
	type best struct {
		sim  float64
		path string
		gold map[string]struct{}
	}
	bests := make([]best, len(scored))
	for i, g := range scored {
		bi, bs := -1, -1.0
		for j, lv := range leafVecs {
			if s := cosine(itemVecs[i], lv); s > bs {
				bi, bs = j, s
			}
		}
		path := ""
		if bi >= 0 {
			if norm, _ := normalize([]string{leaves[bi].Path}); len(norm) > 0 {
				path = norm[0]
			}
		}
		bests[i] = best{sim: bs, path: path, gold: toSet(g.Domain)}
	}

	fmt.Println("── embedding threshold sweep (rule-unresolved set) ──")
	fmt.Printf("%-8s %-10s %-10s %-10s\n", "thresh", "resolved", "hit@any", "acc%")
	for _, th := range []float64{0.30, 0.35, 0.40, 0.45, 0.50, 0.55, 0.60, 0.65, 0.70} {
		resolved, hit := 0, 0
		for _, b := range bests {
			if b.sim < th || b.path == "" {
				continue
			}
			resolved++
			if _, ok := b.gold[b.path]; ok {
				hit++
			}
		}
		fmt.Printf("%-8.2f %-10d %-10d %-9.0f%%\n", th, resolved, hit, pct(hit, resolved))
	}
	fmt.Printf("\n(总 %d 条待兜底；对比运行 C：本地 LLM 全吃 hit@any=71%%)\n", len(scored))
	fmt.Println("读法：找到一档 acc% ≳ 71 且 resolved 仍可观的阈值 → embedding 层留、只放高置信；")
	fmt.Println("       若各档 acc% 都 < 71，或够格的阈值 resolved 极少 → 停用该层，长尾全交 LLM。")
}

// legacy reports whether any learning/* path was among the predictions.
func normalize(paths []string) (norm []string, legacy bool) {
	seen := map[string]struct{}{}
	for _, p := range paths {
		if strings.HasPrefix(p, "learning/") {
			legacy = true
			continue
		}
		if n, ok := aliasOldToNew[p]; ok {
			p = n
		}
		if _, dup := seen[p]; !dup {
			seen[p] = struct{}{}
			norm = append(norm, p)
		}
	}
	return norm, legacy
}

func runEmbedTier(ctx context.Context, embedder provider.Embedder, threshold float64, leaves []taxonomy.Node, metas map[string]itemMeta, pending []goldenItem, results map[string]*result) (still []goldenItem) {
	leafTexts := make([]string, len(leaves))
	for i, n := range leaves {
		leafTexts[i] = fmt.Sprintf("%s: %s — %s", n.Path, n.Name, n.Desc)
	}
	leafVecs, err := embedder.Embed(ctx, leafTexts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "embedding tier failed (%v), passing all to LLM\n", err)
		return pending
	}

	const chunk = 64
	for start := 0; start < len(pending); start += chunk {
		end := min(start+chunk, len(pending))
		batch := pending[start:end]
		texts := make([]string, len(batch))
		for i, g := range batch {
			texts[i] = itemText(metas[g.ID])
		}
		vecs, err := embedder.Embed(ctx, texts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "embed chunk failed (%v), items fall through\n", err)
			still = append(still, batch...)
			continue
		}
		for i, g := range batch {
			bestIdx, bestSim := -1, threshold
			for j, lv := range leafVecs {
				if sim := cosine(vecs[i], lv); sim >= bestSim {
					bestIdx, bestSim = j, sim
				}
			}
			if bestIdx < 0 {
				still = append(still, g)
				continue
			}
			norm, _ := normalize([]string{leaves[bestIdx].Path})
			r := results[g.ID]
			if len(norm) == 0 { // matched a learning/* leaf on the current tree
				r.Tier = "legacy-learning"
				continue
			}
			r.Tier, r.Predicted = "embedding", norm
		}
		fmt.Fprintf(os.Stderr, "embedding: %d/%d scored\n", end, len(pending))
	}
	return still
}

func runLLMTier(ctx context.Context, p provider.Provider, leaves []taxonomy.Node, metas map[string]itemMeta, pending []goldenItem, results map[string]*result, batchSize int) (still []goldenItem) {
	var tree strings.Builder
	for _, n := range leaves {
		fmt.Fprintf(&tree, "- %s (%s: %s)\n", n.Path, n.Name, n.Desc)
	}
	const system = "You are a technical expert who categorizes GitHub repositories. Respond with a single JSON object only."

	for start := 0; start < len(pending); start += batchSize {
		end := min(start+batchSize, len(pending))
		batch := pending[start:end]

		var b strings.Builder
		b.WriteString("For each repository, pick the best category path from the list AND its form type. Use ONLY listed paths.\n\nCategories:\n")
		b.WriteString(tree.String())
		b.WriteString("\ntype is one of: library, app, cli, software (fallback), tutorial, awesome, interview, skill.\n")
		b.WriteString("\nRepositories (echo each id):\n")
		for _, g := range batch {
			m := metas[g.ID]
			fmt.Fprintf(&b, "- id=%q name=%q lang=%q topics=[%s] desc=%q\n",
				g.ID, m.Name, m.Language, strings.Join(m.Topics, ", "), truncate(m.Description, 200))
		}
		b.WriteString("\nRespond: {\"results\":[{\"id\":\"<id>\",\"path\":\"<category path>\",\"type\":\"<type>\"}]}")

		raw, err := p.AnalyzeJSON(ctx, system, b.String())
		if err != nil {
			fmt.Fprintf(os.Stderr, "llm batch failed (%v), items unresolved\n", err)
			still = append(still, batch...)
			continue
		}
		parsed, err := parseLLM(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "llm parse failed (%v), items unresolved\n", err)
			still = append(still, batch...)
			continue
		}
		for _, g := range batch {
			pred, ok := parsed[g.ID]
			if !ok {
				still = append(still, g)
				continue
			}
			r := results[g.ID]
			r.PredictedType = pred.Type
			norm, legacy := normalize([]string{pred.Path})
			switch {
			case len(norm) > 0:
				r.Tier, r.Predicted = "llm", norm
			case legacy:
				r.Tier = "legacy-learning"
			default:
				still = append(still, g)
			}
		}
		fmt.Fprintf(os.Stderr, "llm: %d/%d done\n", end, len(pending))
	}
	return still
}

type llmPred struct {
	Path string
	Type string
}

type llmElem struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Type string `json:"type"`
}

// parseLLM extracts the per-item predictions, tolerating both the wrapped
// {"results":[...]} shape and a bare top-level [...] array (grok sometimes
// returns the latter).
func parseLLM(raw string) (map[string]llmPred, error) {
	s := strings.TrimSpace(raw)
	var elems []llmElem

	// Try the wrapped object first (extract outermost braces if fenced/prosey).
	obj := s
	if i, j := strings.IndexByte(obj, '{'), strings.LastIndexByte(obj, '}'); i >= 0 && j > i {
		obj = obj[i : j+1]
	}
	var w struct {
		Results []llmElem `json:"results"`
	}
	if err := json.Unmarshal([]byte(obj), &w); err == nil && len(w.Results) > 0 {
		elems = w.Results
	} else if i, j := strings.IndexByte(s, '['), strings.LastIndexByte(s, ']'); i >= 0 && j > i {
		// Fall back to a bare array.
		_ = json.Unmarshal([]byte(s[i:j+1]), &elems)
	}

	if len(elems) == 0 {
		return nil, errors.New("no parseable results")
	}
	out := make(map[string]llmPred, len(elems))
	for _, e := range elems {
		if e.ID != "" && e.Path != "" {
			out[e.ID] = llmPred{Path: e.Path, Type: e.Type}
		}
	}
	return out, nil
}

// report prints the summary tables the change-12 baseline needs.
func report(results map[string]*result) {
	var rs []*result
	for _, r := range results {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })

	tierN := map[string]int{}
	tierHit := map[string]int{}
	var tp, fp, fn int
	var withDomain, hitAny int
	docN, docResolved := 0, 0

	for _, r := range rs {
		tierN[r.Tier]++
		goldenSet := toSet(r.Golden)
		predSet := toSet(r.Predicted)
		isDoc := r.GoldenType == "awesome" || r.GoldenType == "interview" || r.GoldenType == "tutorial" || r.GoldenType == "skill"
		if isDoc {
			docN++
			if len(predSet) > 0 {
				docResolved++
			}
		}
		if len(goldenSet) == 0 {
			continue // 资料类允许领域为空：不计入 domain 指标
		}
		withDomain++
		hit := false
		for p := range predSet {
			if _, ok := goldenSet[p]; ok {
				tp++
				hit = true
			} else {
				fp++
			}
		}
		for p := range goldenSet {
			if _, ok := predSet[p]; !ok {
				fn++
			}
		}
		if hit {
			hitAny++
			r.Hit = true
			tierHit[r.Tier]++
		}
	}

	fmt.Println("── eval report ──────────────────────────────")
	fmt.Printf("items scored: %d (with golden domain: %d)\n\n", len(rs), withDomain)
	fmt.Println("tier resolution:")
	for _, t := range []string{"rule", "embedding", "llm", "legacy-learning", "unresolved"} {
		if tierN[t] == 0 {
			continue
		}
		acc := ""
		if t == "rule" || t == "embedding" || t == "llm" {
			acc = fmt.Sprintf("  hit@any=%.0f%%", pct(tierHit[t], tierDomainN(rs, t)))
		}
		fmt.Printf("  %-16s %4d (%.0f%%)%s\n", t, tierN[t], pct(tierN[t], len(rs)), acc)
	}
	prec, rec := safeDiv(tp, tp+fp), safeDiv(tp, tp+fn)
	fmt.Printf("\ndomain (items with golden domain, n=%d):\n", withDomain)
	fmt.Printf("  hit@any:   %.1f%%\n", pct(hitAny, withDomain))
	fmt.Printf("  precision: %.2f  recall: %.2f  (micro, multi-label)\n", prec, rec)
	fmt.Printf("\n资料类 (awesome/interview/tutorial/skill, n=%d):\n", docN)
	fmt.Printf("  domain coverage: %.0f%%   legacy-learning routed: %d\n", pct(docResolved, docN), legacyDocs(rs))
	reportLLMType(rs)
}

// reportLLMType measures LLM-emitted type accuracy on the items the LLM typed
// (the rule-unresolved set — the hard cases that lack topic signals). Answers
// whether the software sub-form (cli/app/library) can piggyback on the LLM tier.
func reportLLMType(rs []*result) {
	var n, exact, binHit int
	confusion := map[string]int{}
	isDoc := func(t string) bool {
		return t == "awesome" || t == "interview" || t == "tutorial" || t == "skill"
	}
	for _, r := range rs {
		if r.PredictedType == "" {
			continue
		}
		n++
		if r.PredictedType == r.GoldenType {
			exact++
		} else {
			confusion[fmt.Sprintf("%s→%s", r.GoldenType, r.PredictedType)]++
		}
		if isDoc(r.GoldenType) == isDoc(r.PredictedType) {
			binHit++
		}
	}
	if n == 0 {
		fmt.Println("\ntype facet: LLM emitted no type this run")
		return
	}
	fmt.Printf("\nLLM type accuracy (rule-unresolved set, n=%d):\n", n)
	fmt.Printf("  exact:  %.0f%%\n", pct(exact, n))
	fmt.Printf("  资料/软件 二值: %.0f%%\n", pct(binHit, n))
	type kv struct {
		k string
		v int
	}
	var top []kv
	for k, v := range confusion {
		top = append(top, kv{k, v})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].v > top[j].v })
	for i, x := range top {
		if i >= 6 || x.v < 2 {
			break
		}
		fmt.Printf("    %-22s %d\n", x.k, x.v)
	}
}

func tierDomainN(rs []*result, tier string) int {
	n := 0
	for _, r := range rs {
		if r.Tier == tier && len(r.Golden) > 0 {
			n++
		}
	}
	return n
}

func legacyDocs(rs []*result) int {
	n := 0
	for _, r := range rs {
		isDoc := r.GoldenType == "awesome" || r.GoldenType == "interview" || r.GoldenType == "tutorial" || r.GoldenType == "skill"
		if isDoc && r.Tier == "legacy-learning" {
			n++
		}
	}
	return n
}

// ── helpers ─────────────────────────────────────────────────────────────

func loadGolden(path string) ([]goldenItem, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f goldenFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	return f.Items, nil
}

// leafNodes returns nodes that are no other node's parent (assignment targets).
func leafNodes(nodes []taxonomy.Node) []taxonomy.Node {
	hasChild := map[string]bool{}
	for _, n := range nodes {
		if i := strings.LastIndex(n.Path, "/"); i > 0 {
			hasChild[n.Path[:i]] = true
		}
	}
	var leaves []taxonomy.Node
	for _, n := range nodes {
		if !hasChild[n.Path] {
			leaves = append(leaves, n)
		}
	}
	return leaves
}

func fetchItem(apiBase, id string) (itemMeta, error) {
	u := fmt.Sprintf("%s/trending/item?source=github&externalId=%s", apiBase, url.QueryEscape(id))
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		resp, err := http.Get(u)
		if err != nil {
			lastErr = err
			continue
		}
		var body struct {
			Data struct {
				Item struct {
					Name        string         `json:"name"`
					Description string         `json:"description"`
					Language    string         `json:"language"`
					SourceData  map[string]any `json:"sourceData"`
				} `json:"item"`
			} `json:"data"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return itemMeta{}, errors.New("not found")
		}
		if err != nil || resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status=%d err=%v", resp.StatusCode, err)
			continue
		}
		it := body.Data.Item
		var topics []string
		if raw, ok := it.SourceData["topicNames"].([]any); ok {
			for _, t := range raw {
				if s, ok := t.(string); ok {
					topics = append(topics, s)
				}
			}
		}
		return itemMeta{Name: it.Name, Description: it.Description, Language: it.Language, Topics: topics}, nil
	}
	return itemMeta{}, lastErr
}

func envConfig() *config.Config {
	threshold := 0.35
	if v := os.Getenv("EMBED_SIM_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			threshold = f
		}
	}
	return &config.Config{
		AIProvider:        getenv("AI_PROVIDER", "openai"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:       getenv("OPENAI_MODEL", "gpt-4o-mini"),
		LMStudioBaseURL:   getenv("LMSTUDIO_BASE_URL", "http://localhost:1234/v1"),
		LMStudioModel:     os.Getenv("LMSTUDIO_LOCAL_MODULE_NAME"),
		EmbedModel:        os.Getenv("EMBED_MODEL"),
		EmbedSimThreshold: threshold,
	}
}

func itemText(m itemMeta) string {
	parts := []string{m.Name}
	if m.Description != "" {
		parts = append(parts, m.Description)
	}
	if m.Language != "" {
		parts = append(parts, m.Language)
	}
	if len(m.Topics) > 0 {
		parts = append(parts, strings.Join(m.Topics, ", "))
	}
	return truncate(strings.Join(parts, ". "), 800)
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return -1
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return -1
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

func writeJSON(path string, results map[string]*result) {
	var rs []*result
	for _, r := range results {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })
	data, _ := json.MarshalIndent(rs, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) * 100 / float64(b)
}

func safeDiv(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
		os.Exit(1)
	}
}
