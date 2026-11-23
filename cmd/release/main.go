// Command release is the testable half of scripts/release.sh: it computes
// date versions, release notes and the release PR body. It runs no git, gh or
// ssh itself; the script passes it what those return.
//
//	release next-version [-date YYYY-MM-DD] < tags      next vYYYY.MM.DD.N for that UTC day
//	release latest < tags                                highest date version, exit 1 if none
//	release notes -version V -services a,b [-limit N] < gitlog  release notes (markdown)
//	release promote-body -version V < gitlog             body of the develop → main PR
//	release window [-at RFC3339]                         exit 0 inside UTC 03:30–08:30, 1 outside
//
// tags is one tag per line; gitlog is `git log --merges --format=<LogFormat>`,
// with LogFormat as release.LogFormat ("%s%x1f%b%x1e").
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/elbaldfun/ghta/internal/release"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	switch cmd {
	case "next-version":
		date := fs.String("date", "", "UTC day, YYYY-MM-DD (default today)")
		_ = fs.Parse(args)
		day := time.Now().UTC()
		if *date != "" {
			d, err := time.Parse("2006-01-02", *date)
			if err != nil {
				fail("bad -date: %v", err)
			}
			day = d
		}
		fmt.Println(release.NextVersion(day, lines(stdin())))
	case "latest":
		_ = fs.Parse(args)
		v, ok := release.Latest(lines(stdin()))
		if !ok {
			os.Exit(1)
		}
		fmt.Println(v)
	case "notes":
		version := fs.String("version", "", "version tag")
		services := fs.String("services", "", "comma-separated services")
		limit := fs.Int("limit", 0, "keep only the newest N PRs (0: all)")
		_ = fs.Parse(args)
		if *version == "" || *services == "" {
			fail("notes needs -version and -services")
		}
		prs := release.ParseLog(stdin())
		if *limit > 0 && len(prs) > *limit {
			prs = prs[:*limit] // ParseLog is newest first
		}
		fmt.Print(release.Notes(*version, strings.Split(*services, ","), prs))
	case "promote-body":
		version := fs.String("version", "", "expected version")
		_ = fs.Parse(args)
		if *version == "" {
			fail("promote-body needs -version")
		}
		fmt.Print(release.PromoteBody(*version, release.ParseLog(stdin())))
	case "window":
		at := fs.String("at", "", "time to check, RFC3339 (default now)")
		_ = fs.Parse(args)
		t := time.Now()
		if *at != "" {
			p, err := time.Parse(time.RFC3339, *at)
			if err != nil {
				fail("bad -at: %v", err)
			}
			t = p
		}
		if !release.InPipelineWindow(t) {
			os.Exit(1)
		}
	default:
		usage()
	}
}

func stdin() string {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail("read stdin: %v", err)
	}
	return string(b)
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: release next-version|latest|notes|promote-body|window [flags]")
	os.Exit(2)
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "release: "+format+"\n", a...)
	os.Exit(2)
}
