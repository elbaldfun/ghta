package boards

// Board keys. The job writes under them and the api reads them back; a key
// names one whole board.

func DevelopersKey(board, group string) string { return "developers|" + board + "|" + group }

func TopicsKey(sort, group string) string { return "topics|" + sort + "|" + group }

func EcosystemKey(pillar, sort string) string { return "ecosystem|" + pillar + "|" + sort }

const (
	HeatKey       = "heat"
	CategoriesKey = "categories"
	TypeFacetsKey = "facets|type"
	LanguagesKey  = "stats|languages"
	StalenessKey  = "stats|staleness"
)

// HistoryHead is how many leading ids of a ranked board boards_history keeps
// each day.
const HistoryHead = 100
