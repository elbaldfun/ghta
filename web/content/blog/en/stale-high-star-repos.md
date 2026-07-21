---
title: "A Third of GitHub's Most-Starred Repos Have Been Cold for Two Years"
description: "20,435 of the high-star repositories we track have had no push in over two years. The obvious explanations — they're all awesome-lists, they're just finished — turn out to explain less than you'd think."
date: 2026-07-22
tags: [data, maintenance]
---

We track more than 67,000 GitHub repositories with a thousand stars or more, and record their state daily. Sorting them by how long it has been since the last push produces an uncomfortable number.

```starrank:staleness
```

Roughly a third have not received a single push in over two years. Under half have been touched in the last three months.

That number invites two immediate objections. Both are reasonable. Neither survives the data intact.

## Objection 1: "Those are all awesome-lists"

The intuition is that curated lists, book collections and interview-prep repos hoard stars without ever needing a commit. They inflate the stale bucket while telling us nothing about software.

It's a real effect. Repositories with no detected programming language make up 5.0% of the actively-maintained bucket and 10.8% of the two-year-cold bucket — twice the share. Add documentation-shaped languages (Markdown, HTML, TeX, Jupyter) and the pattern holds: 3.4% versus 6.0%.

But put together, that's about a sixth of the cold bucket. **The other five sixths are repositories with a real programming language attached** — actual software, sitting untouched for two years with thousands of stars pointing at it.

Here are the largest of them:

```starrank:stale-repos
limit: 10
```

Some are genuinely reference material. Others are software people still install.

## Objection 2: "Stale isn't abandoned — good software gets finished"

This one is stronger. A focused library that solved its problem correctly does not need commits. Churn is not health, and a quiet repo may just be a done repo.

If that were the whole story, though, we'd expect a signature in the issue tracker. An abandoned project with real users accumulates issues nobody triages; a genuinely finished project doesn't attract many in the first place. So the backlog, normalised against audience size, should look very different between the two groups.

It doesn't. Look at the last column of the table above: open issues per thousand stars sits between 9 and 11.5 across **every** bucket. Actively maintained projects carry a marginally *higher* normalised backlog than two-year-cold ones.

That result surprised us — we built the column expecting it to separate the groups, and it refused to.

## What the flat line probably means

The most plausible reading is that abandonment is mutual. Projects don't usually go quiet while users keep hammering the issue tracker. Attention leaves from both sides at once: the maintainer stops pushing, and the people who would have filed issues have already moved to something else.

That's a less dramatic story than "thousands of neglected projects with furious users." It's also worse news for anyone using GitHub stars to pick dependencies. A repository can be simultaneously highly starred, visibly quiet, and *not obviously broken from the outside* — because the users who would have complained left without saying anything.

We should be careful about how far to push this. What we can see is one current snapshot of open-issue counts, and GitHub's number folds pull requests in with issues. We can't see whether issues were closed en masse, whether the maintainer turned the tracker off, or how the backlog moved over time. The flat line is consistent with mutual abandonment; it does not prove it.

## The practical version

Star count records how many people once thought a project was worth bookmarking. It says nothing about whether anyone still maintains it, and — going by the issue data — not much about whether anyone still uses it either.

Before adopting a dependency on the strength of its star count, look at the last push date. It's one click away and it disagrees with the star count about a third of the time.

## What this data can't tell you

- **The sample is repositories that already cleared 1,000 stars**, not all of GitHub. Nothing here describes typical repositories.
- **`pushedAt` counts pushes to any branch**, including automated commits. It's a liveness signal, not a measure of meaningful work.
- **Open-issue counts include pull requests**, and we hold a single current snapshot rather than a history.
- **Language attribution follows GitHub's own detection**, which decides by file bytes and can label a project unexpectedly.
- 4,123 tracked repositories have no usable push date recorded and sit outside every bucket above.

The tables update with our daily crawl. You can sort the same corpus yourself on the [rankings](/).
