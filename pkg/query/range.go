// Package query parses user-facing filter/sort expressions into Mongo queries.
package query

import (
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

// ParseRange turns a range expression into a Mongo condition for one numeric
// field. Supported forms: "a..b" (inclusive), ">n", "<n", "n" (exact).
// Zero is a valid boundary. Empty expr yields no condition.
func ParseRange(expr string) (bson.M, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}

	switch {
	case strings.Contains(expr, ".."):
		parts := strings.SplitN(expr, "..", 2)
		lo, err1 := parseNum(parts[0])
		hi, err2 := parseNum(parts[1])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("invalid range %q", expr)
		}
		return bson.M{"$gte": lo, "$lte": hi}, nil

	case strings.HasPrefix(expr, ">"):
		n, err := parseNum(expr[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid range %q", expr)
		}
		return bson.M{"$gt": n}, nil

	case strings.HasPrefix(expr, "<"):
		n, err := parseNum(expr[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid range %q", expr)
		}
		return bson.M{"$lt": n}, nil

	default:
		n, err := parseNum(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", expr)
		}
		return bson.M{"$eq": n}, nil
	}
}

func parseNum(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}
