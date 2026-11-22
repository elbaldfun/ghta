package api

import (
	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
)

func (a *API) registerModels(r gin.IRouter) {
	r.GET("/models/:author/:name", a.model)
}

// ModelDetail is a model's board row with the lineage a page shows beside
// it: what it was made from (BaseModel) and how many tracked models were
// made from it.
type ModelDetail struct {
	boards.Model
	// Derivatives counts the live tracked models that name this one as a
	// base: the boards' corpus (the Hub's top models by downloads, likes and
	// trend), not every model on the Hub. Absent when it could not be read.
	Derivatives *int     `json:"derivatives"`
	Datasets    []string `json:"datasets"`
	Arxiv       []string `json:"arxiv"`
	Languages   []string `json:"languages"`
}

// GET /v1/models/{author}/{name} — one HuggingFace model; 404 for one not
// tracked or gone.
func (a *API) model(c *gin.Context) {
	ctx := c.Request.Context()
	it, err := a.st.Item(ctx, item.SourceHuggingFace, c.Param("author")+"/"+c.Param("name"))
	if err != nil {
		a.fail(c, notFound(err))
		return
	}
	row, ok := boards.ModelOf(boards.ModelRow{ExternalID: it.ExternalID, Name: it.Name, License: it.License,
		Metrics: it.Metrics, Growth: it.Growth, Hf: it.Hf, Stale: it.Stale})
	if !ok {
		a.fail(c, errNotFound)
		return
	}
	d := ModelDetail{Model: row, Datasets: orEmpty(it.Hf.Datasets), Arxiv: orEmpty(it.Hf.Arxiv),
		Languages: orEmpty(it.Hf.Languages)}
	if n, err := a.st.Derivatives(ctx, it.ExternalID); err != nil {
		a.log.Warn("api: model derivatives", "model", it.ExternalID, "err", err)
	} else {
		d.Derivatives = &n
	}
	a.cached(c, maxAgeDetail, gin.H{"data": d})
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
