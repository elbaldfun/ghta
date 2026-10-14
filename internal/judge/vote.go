package judge

import (
	"slices"

	"github.com/elbaldfun/ghta/internal/item"
)

// Confidence says how the votes on an item fell.
const (
	// Agreed: the first two answers matched. On the golden set that is nine
	// items in ten, right about nine times in ten.
	Agreed = "agreed"
	// Majority: the first two differed and a third settled it.
	Majority = "majority"
	// Split: three answers and still no majority on some field. The item
	// keeps the first answer for the domain and the form, stays off the store
	// until reviewed (an unsettled shelf is no shelf), and is found for review
	// by this value.
	Split = "split"
)

// MaxVotes is how many answers an item gets at most: two, and a third only
// when those disagree.
const MaxVotes = 3

// Decision is what the votes on one item add up to.
type Decision struct {
	Verdict    Verdict
	Confidence string
	Votes      int
}

// Decide combines the answers given for one item. votes[0] must be a full
// answer: the taglines, alternatives and tags are taken from it, since prose
// cannot be voted on.
//
// The votes are compared on the primary domain, the form and the shelf. more
// reports that they disagree and another answer is worth asking for; it is
// never true once MaxVotes are in.
func Decide(votes []Verdict) (d Decision, more bool) {
	if len(votes) == 0 {
		return Decision{}, true
	}
	first := votes[0]
	d = Decision{Verdict: first, Votes: len(votes)}
	if len(votes) == 1 {
		return d, true
	}
	primary, okPrimary := majority(votes, Verdict.Primary)
	typ, okType := majority(votes, func(v Verdict) string { return v.Type })
	shelf, okShelf := majority(votes, func(v Verdict) string { return v.Shelf })
	if !(okPrimary && okType && okShelf) && len(votes) < MaxVotes {
		return d, true
	}

	if okType {
		d.Verdict.Type = typ
	}
	if okShelf {
		d.Verdict.Shelf = shelf
	} else {
		// Unsure where it goes: kept off the store rather than on a doubtful shelf.
		d.Verdict.Shelf = item.ShelfExcluded
	}
	if !okPrimary {
		primary = first.Primary()
	}
	// The domain keeps the leaves most answers named, the primary leading.
	var paths []string
	if primary != "" {
		paths = append(paths, primary)
	}
	for _, v := range votes {
		for _, p := range v.Paths {
			if !slices.Contains(paths, p) && count(votes, p)*2 > len(votes) {
				paths = append(paths, p)
			}
		}
	}
	d.Verdict.Paths = paths

	switch {
	case !(okPrimary && okType && okShelf):
		d.Confidence = Split
	case len(votes) == 2:
		d.Confidence = Agreed
	default:
		d.Confidence = Majority
	}
	return d, false
}

// majority returns the value more than half of the votes share.
func majority(votes []Verdict, key func(Verdict) string) (string, bool) {
	tally := map[string]int{}
	for _, v := range votes {
		k := key(v)
		tally[k]++
		if tally[k]*2 > len(votes) {
			return k, true
		}
	}
	return "", false
}

func count(votes []Verdict, leaf string) int {
	n := 0
	for _, v := range votes {
		if slices.Contains(v.Paths, leaf) {
			n++
		}
	}
	return n
}
