package judge

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elbaldfun/ghta/internal/taxonomy"
)

const systemPrompt = "You are a technical expert who catalogues open-source GitHub repositories. Respond with a single JSON object only."

// promptHead is the part of the prompt that never changes between calls. It
// comes first so the server can reuse its prompt cache for it.
func promptHead(leaves, shelves []taxonomy.Node) string {
	var b strings.Builder
	b.WriteString("Judge each repository below.\n\nDomain leaf paths (path — name: what belongs there):\n")
	for _, n := range leaves {
		fmt.Fprintf(&b, "- %s — %s: %s\n", n.Path, n.Name, n.Desc)
	}
	b.WriteString("\nConsumer store shelves (slug — name: what belongs there):\n")
	for _, n := range shelves {
		fmt.Fprintf(&b, "- %s — %s: %s\n", taxonomy.ShelfSlug(n.Path), n.Name, n.Desc)
	}
	return b.String()
}

var fieldsCore = `
For each repository return:
- paths: 1-3 domain leaf paths, most relevant first: the technical field the repository belongs to, whatever its
  form. A repository may span domains. Answer this independently of the shelf.
- type: its form.
  library = code other developers build on: libraries, frameworks (web and app frameworks, static-site generators,
    build tools, UI kits), SDKs, and machine-learning models or inference/training toolkits used from code;
  app = an application an end user operates through a graphical or web UI, self-hosted web applications included;
  cli = a tool used mainly from the command line;
  tutorial = self-contained teaching content; awesome = a list of links; interview = interview/algorithm practice;
  skill = ONLY a packaged skill or prompt pack installed into an AI coding assistant;
  software = a standalone system that is run rather than imported and is not operated through its own UI: servers,
    databases, language runtimes and compilers, operating systems, infrastructure daemons, firmware, protocol nodes.
- shelf: the ONE shelf of an app store of downloadable, runnable software it belongs on.
  Either a consumer shelf from the list above (software a non-developer would install), or a developer shelf, which
  is one of these domain leaves: ` + "`" + strings.Join(DevShelves, "`, `") + "`" + `
  (terminals, editors, API clients, database clients, git tools, package managers, linters, container/CI/monitoring
  tools, game engines with an editor, offensive-security tools, coding agents).
  Answer "excluded" unless a user can download, run or self-host it: frameworks, libraries, SDKs, lists, tutorials,
  interview collections, datasets, specs and hosted-only services are excluded. Command-line tools ARE apps.
`

const fieldsFull = `- hasGui: true if it has a graphical or web UI a user interacts with; false if command-line or daemon only.
- taglineZh: ONE plain-Chinese sentence (at most 20 characters) telling a normal user what it does for them.
  Plain words, no hype, no "开源" prefix. Example: "跨平台截图与标注工具".
- taglineEn: the same in plain English (at most 70 characters).
- alternatives: the well-known commercial or paid products it is a direct, like-for-like replacement for, by
  canonical name ("Notion", "Figma", "Postman", "Adobe Photoshop"), with a short kind ("note-taking", "design").
  At most 3. Most repositories replace no paid product: an empty array is the common, correct answer.
  Never list open-source projects and never invent products.
- tags: 3-6 short lowercase keywords.
`

const fieldsRules = `Rules:
1. Use the name, description, language, topics and README excerpt. Choose the most specific leaf.
2. paths may be empty for a resource repository (awesome / tutorial / interview) no domain fits, and for a
   consumer application whose shelf already says everything about it.
3. Return exactly one result for every id.

Repositories:
`

func fields(full bool) string {
	if full {
		return fieldsCore + fieldsFull + fieldsRules
	}
	return fieldsCore + fieldsRules
}

// lines renders the batch, one repository per line.
func lines(batch []Input) string {
	var b strings.Builder
	for _, in := range batch {
		topics := in.Topics
		if len(topics) > 12 {
			topics = topics[:12]
		}
		fmt.Fprintf(&b, "- id=%s name=%s lang=%s topics=[%s] desc=%s",
			quote(in.ID), quote(in.Name), quote(in.Language), strings.Join(topics, ", "), quote(clip(in.Description, 240)))
		if len(in.Platforms) > 0 {
			fmt.Fprintf(&b, " platforms=[%s]", strings.Join(in.Platforms, ", "))
		}
		if in.Readme != "" {
			fmt.Fprintf(&b, " readme=%s", quote(in.Readme))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// object is a JSON object that keeps its keys in the order given. A schema's
// property order is the order the model writes in, so it cannot be a Go map.
type object []field

type field struct {
	key   string
	value any
}

func (o object) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		v, err := json.Marshal(f.value)
		if err != nil {
			return nil, err
		}
		b.WriteString(quote(f.key))
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// schema constrains the reply: one result per id of the batch, categories and
// shelves only from the tree. The judgement fields come before the prose.
func (j *Judge) schema(ids []string, full bool) json.RawMessage {
	str := object{{"type", "string"}}
	enum := func(values []string) object { return object{{"type", "string"}, {"enum", values}} }
	list := func(items any, max int) object {
		return object{{"type", "array"}, {"items", items}, {"minItems", 0}, {"maxItems", max}}
	}
	props := object{
		{"id", enum(ids)},
		{"paths", list(enum(j.leaves), maxDomain)},
		{"type", enum(Types)},
		{"shelf", enum(j.shelves)},
	}
	if full {
		props = append(props,
			field{"hasGui", object{{"type", "boolean"}}},
			field{"taglineZh", object{{"type", "string"}, {"maxLength", taglineZhRunes}}},
			field{"taglineEn", object{{"type", "string"}, {"maxLength", taglineEnRunes}}},
			field{"alternatives", list(object{{"type", "object"},
				{"properties", object{{"name", str}, {"kind", str}}}, {"required", []string{"name", "kind"}}}, maxAlternatives)},
			field{"tags", list(str, maxTags)},
		)
	}
	required := make([]string, len(props))
	for i, f := range props {
		required[i] = f.key
	}
	raw, _ := json.Marshal(object{{"type", "object"}, {"required", []string{"results"}}, {"properties", object{
		{"results", object{{"type", "array"}, {"minItems", len(ids)}, {"maxItems", len(ids)},
			{"items", object{{"type", "object"}, {"properties", props}, {"required", required}}}}},
	}}})
	return raw
}

// Excerpt returns the first n runes of a README with markdown noise (badges,
// headings, tables, html) dropped: enough to tell what the project is.
func Excerpt(readme string, n int) string {
	var b strings.Builder
	for _, line := range strings.Split(readme, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "<") || strings.HasPrefix(l, "[!") ||
			strings.HasPrefix(l, "!") || strings.HasPrefix(l, "|") || strings.HasPrefix(l, "---") {
			continue
		}
		b.WriteString(l)
		b.WriteByte(' ')
		if b.Len() > n*4 { // a byte bound that always covers n runes
			break
		}
	}
	return clip(strings.TrimSpace(b.String()), n)
}
