---
title: "Count Isn't Weight: What 67,000 Repos Say About Language Ecosystems"
description: "We track 67,000 high-star GitHub repositories daily. Split by language, the one with the most projects turns out not to be the one with the strongest typical project."
date: 2026-07-21
tags: [data, ecosystems]
---

The usual way to compare language ecosystems is to ask which language has the most projects. It's an easy question to answer and a weak one to act on — it tells you about **count**, not about **weight**.

We track more than 67,000 high-star GitHub repositories and record how their stars move every day. Splitting that corpus by language shows something more useful than a headcount.

## The raw numbers

This table is read live from our database on every page load — it isn't a screenshot:

```starrank:languages
limit: 10
```

## Three things worth noticing

### 1. The leader by count doesn't lead by median

Python is far ahead on repository count — roughly 1.4× the runner-up. But look at the **median** star count, which answers "if I pick a random project in this language, how popular is it likely to be": Python sits behind both Go and TypeScript.

That's a signal of **breadth**. Python covers scientific computing, scraping, machine learning, automation — almost everything. The flip side of breadth is a very long tail, with many projects sitting just above the tracking threshold.

### 2. Go has the highest hit rate

Go has about a third of Python's repository count, yet the highest median of any language here.

The plausible explanation is **domain concentration**. Go's popular projects cluster into cloud-native infrastructure, DevOps and CLI tooling — fields whose users are themselves developers, and therefore people who star things. Much of Python's reach is into audiences that never open GitHub at all.

### 3. Total stars and median tell different stories

JavaScript's total star count is high while its median is comparatively low. The total reflects a cohort of very large, very old projects — that's **accumulated history**. The soft median suggests a smaller share of recent projects break through.

TypeScript inverts this: fewer repositories than JavaScript but a higher median, consistent with newer projects defaulting to TypeScript from day one.

## The top of the list

Composition matters as much as counts. These are the highest-starred repositories we track right now:

```starrank:top-repos
limit: 10
```

Notice how many of them are **learning resources and curated lists** rather than runnable software. Those earn enormous star counts while saying little about a language's engineering ecosystem — lumping them in with frameworks and runtimes distorts any comparison.

## What this data can't tell you

Any analysis should state its own limits:

- **The sample is already-popular projects**, not all of GitHub. The medians here are medians *among projects that already cleared the bar*, far above the true site-wide figure.
- **Stars aren't quality**, and they aren't usage. They approximate developer visibility. Plenty of critical enterprise libraries carry star counts well below their real importance.
- **Language attribution follows GitHub's primary-language detection.** A frontend framework can be labelled something unexpected because of how its file bytes shake out.
- **Ecosystems differ in age.** C and Java have decades of accumulation; Rust has had far less time. Comparing totals directly is unfair to the younger ones.

## Takeaway

If you're choosing a stack, a better question than "which language has more projects" is: **how many mature options exist in the specific niche I need**. Global counts help surprisingly little with that.

The table above updates itself with our daily crawl. You can slice the same data by language and category yourself on the [rankings](/).
