package github

import (
	"encoding/json"
	"testing"
)

// A trimmed real /users/{login} response, guarding the field mapping (GitHub's
// snake_case -> our struct) against silent drift.
const sampleUserJSON = `{
  "login": "sindresorhus",
  "type": "User",
  "name": "Sindre Sorhus",
  "company": null,
  "blog": "https://sindresorhus.com/apps",
  "location": null,
  "bio": "Full-Time Open-Sourcerer.",
  "twitter_username": "sindresorhus",
  "followers": 80425,
  "following": 31,
  "public_repos": 1140,
  "avatar_url": "https://avatars.githubusercontent.com/u/170270?v=4",
  "created_at": "2009-12-20T22:57:02Z"
}`

func TestUserProfileDecode(t *testing.T) {
	var u UserProfile
	if err := json.Unmarshal([]byte(sampleUserJSON), &u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.Login != "sindresorhus" {
		t.Errorf("login = %q", u.Login)
	}
	if u.Type != "User" {
		t.Errorf("type = %q, want User", u.Type)
	}
	if u.TwitterUsername != "sindresorhus" {
		t.Errorf("twitterUsername = %q (snake_case field mapping broke?)", u.TwitterUsername)
	}
	if u.Followers != 80425 {
		t.Errorf("followers = %d", u.Followers)
	}
	if u.PublicRepos != 1140 {
		t.Errorf("publicRepos = %d (public_repos mapping broke?)", u.PublicRepos)
	}
	if u.CreatedAt.Year() != 2009 {
		t.Errorf("createdAt year = %d, want 2009", u.CreatedAt.Year())
	}
	// A null JSON field must decode to the zero value, not error.
	if u.Company != "" || u.Location != "" {
		t.Errorf("null fields should be empty: company=%q location=%q", u.Company, u.Location)
	}
}
