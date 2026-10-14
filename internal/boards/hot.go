package boards

// GitHub's own trending lists, scraped nightly and joined to what we know of
// each repo. Windows are GitHub's: daily, weekly, monthly.
var HotWindows = []string{"daily", "weekly", "monthly"}

func HotKey(window string) string { return "hot|" + window }

// HotRepo is one row of a trending list, in GitHub's order.
type HotRepo struct {
	FullName    string `bson:"fullName" json:"fullName"`
	Owner       string `bson:"owner" json:"owner"`
	Name        string `bson:"name" json:"name"`
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	Language    string `bson:"language,omitempty" json:"language,omitempty"`
	Stars       int    `bson:"stars" json:"stars"`
	Forks       int    `bson:"forks" json:"forks"`
	// StarsGained is what GitHub says the repo gained in the window.
	StarsGained int `bson:"starsGained" json:"starsGained"`
	// Tracked is false for a repo GitHub lists that we could not take in.
	Tracked   bool     `bson:"tracked" json:"tracked"`
	Domain    []string `bson:"domain,omitempty" json:"domain,omitempty"`
	Type      string   `bson:"type,omitempty" json:"type,omitempty"`
	TaglineZh string   `bson:"taglineZh,omitempty" json:"taglineZh,omitempty"`
	TaglineEn string   `bson:"taglineEn,omitempty" json:"taglineEn,omitempty"`
	IconURL   string   `bson:"iconUrl,omitempty" json:"iconUrl,omitempty"`
}
