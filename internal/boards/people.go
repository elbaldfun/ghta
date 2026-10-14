package boards

import (
	"math"
	"sort"
)

// Developer boards rank individuals (never organisations) by the repos they
// own: "rising" by the day's star gain with a track-record floor, "merit" by
// log-summed stars, which rewards a body of work over one lucky hit.
const (
	DevRising = "rising"
	DevMerit  = "merit"

	// DevTopN is how deep a developer board goes.
	DevTopN = 500
	// risingStarFloor keeps a brand-new repo's one-week spike off the top.
	risingStarFloor = 2000
)

// Profile is what the boards need of a developers document.
type Profile struct {
	Login           string `bson:"login"`
	Type            string `bson:"type"`
	Name            string `bson:"name"`
	TwitterUsername string `bson:"twitterUsername"`
	Followers       int    `bson:"followers"`
	Company         string `bson:"company"`
	AvatarURL       string `bson:"avatarUrl"`
	Bio             string `bson:"bio"`
}

// profileUser is the developers.type of an individual.
const profileUser = "User"

type Developer struct {
	Login           string  `bson:"login" json:"login"`
	Name            string  `bson:"name,omitempty" json:"name,omitempty"`
	TwitterUsername string  `bson:"twitterUsername,omitempty" json:"twitterUsername,omitempty"`
	Followers       int     `bson:"followers" json:"followers"`
	Company         string  `bson:"company,omitempty" json:"company,omitempty"`
	AvatarURL       string  `bson:"avatarUrl,omitempty" json:"avatarUrl,omitempty"`
	Bio             string  `bson:"bio,omitempty" json:"bio,omitempty"`
	RepoCount       int     `bson:"repoCount" json:"repoCount"`
	TotalStars      int     `bson:"totalStars" json:"totalStars"`
	TopRepo         string  `bson:"topRepo,omitempty" json:"topRepo,omitempty"`
	TopRepoStars    int     `bson:"topRepoStars" json:"topRepoStars"`
	Growth          int     `bson:"growth" json:"growth"`
	Score           float64 `bson:"score" json:"score"`
}

// Developers returns the rising and merit boards of one domain group.
func (c *Corpus) Developers(g Group, profiles map[string]Profile) (rising, merit []Developer) {
	type acc struct {
		stars, merit, growth float64
		repos                int
		top                  top
	}
	byOwner := map[string]*acc{}
	for i := range c.rows {
		r := &c.rows[i]
		if r.Gh == nil || r.Gh.Owner == "" || !g.has(r) {
			continue
		}
		a := byOwner[r.Gh.Owner]
		if a == nil {
			a = &acc{}
			byOwner[r.Gh.Owner] = a
		}
		a.stars += r.stars()
		a.merit += math.Log10(math.Max(r.stars(), 1))
		a.growth += r.d1()
		a.repos++
		a.top.offer(r)
	}

	for login, a := range byOwner {
		p, ok := profiles[login]
		if !ok || p.Type != profileUser {
			continue
		}
		d := Developer{
			Login: login, Name: p.Name, TwitterUsername: p.TwitterUsername, Followers: p.Followers,
			Company: p.Company, AvatarURL: p.AvatarURL, Bio: p.Bio,
			RepoCount: a.repos, TotalStars: int(a.stars), TopRepo: a.top.ID, TopRepoStars: int(a.top.Stars),
			Growth: int(a.growth),
		}
		d.Score = a.merit
		merit = append(merit, d)
		if a.stars >= risingStarFloor {
			d.Score = a.growth
			rising = append(rising, d)
		}
	}
	return headDevelopers(rising), headDevelopers(merit)
}

func headDevelopers(rows []Developer) []Developer {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].Login < rows[j].Login
	})
	if len(rows) > DevTopN {
		rows = rows[:DevTopN]
	}
	if rows == nil {
		rows = []Developer{}
	}
	return rows
}

// Topic boards rank GitHub topics: "hot" by the day's star gain summed over
// the repos carrying the topic, "popular" by how many repos carry it.
const (
	TopicHot     = "hot"
	TopicPopular = "popular"

	TopicTopN = 60
	// topicRepoFloor drops topics on too few repos to mean anything.
	topicRepoFloor = 5
)

type Topic struct {
	Topic        string  `bson:"topic" json:"topic"`
	RepoCount    int     `bson:"repoCount" json:"repoCount"`
	TotalStars   int     `bson:"totalStars" json:"totalStars"`
	Growth       int     `bson:"growth" json:"growth"`
	TopRepo      string  `bson:"topRepo,omitempty" json:"topRepo,omitempty"`
	TopRepoStars int     `bson:"topRepoStars" json:"topRepoStars"`
	Score        float64 `bson:"score" json:"score"`
}

// Topics returns the hot and popular boards of one domain group.
func (c *Corpus) Topics(g Group) (hot, popular []Topic) {
	type acc struct {
		stars, growth float64
		repos         int
		top           top
	}
	byTopic := map[string]*acc{}
	for i := range c.rows {
		r := &c.rows[i]
		if !g.has(r) {
			continue
		}
		for _, t := range r.Topics {
			a := byTopic[t]
			if a == nil {
				a = &acc{}
				byTopic[t] = a
			}
			a.stars += r.stars()
			a.growth += r.d1()
			a.repos++
			a.top.offer(r)
		}
	}
	for name, a := range byTopic {
		if a.repos < topicRepoFloor {
			continue
		}
		t := Topic{Topic: name, RepoCount: a.repos, TotalStars: int(a.stars), Growth: int(a.growth),
			TopRepo: a.top.ID, TopRepoStars: int(a.top.Stars)}
		t.Score = a.growth
		hot = append(hot, t)
		t.Score = float64(a.repos)
		popular = append(popular, t)
	}
	return headTopics(hot), headTopics(popular)
}

func headTopics(rows []Topic) []Topic {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].Topic < rows[j].Topic
	})
	if len(rows) > TopicTopN {
		rows = rows[:TopicTopN]
	}
	if rows == nil {
		rows = []Topic{}
	}
	return rows
}
