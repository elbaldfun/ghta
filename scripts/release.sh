#!/usr/bin/env bash
# Release starrank.dev to the production droplet.
#
#   scripts/release.sh api            build + ship + restart the Go API
#   scripts/release.sh web            build + ship + restart the Next.js frontend
#   scripts/release.sh worker         build + ship + restart the job worker (it
#                                     shares the api image)
#   scripts/release.sh all            api, worker, then web (backend
#                                     first: the frontend may send params an
#                                     older API answers 400 to)
#   scripts/release.sh compose        sync deploy/docker-compose.yml, recreate what changed
#   scripts/release.sh site           publish deploy/starrank.caddy to the shared Caddy
#   scripts/release.sh rollback api|web   put the previous image back (api
#                                     takes the worker back with it)
#   scripts/release.sh status         what is live right now
#   scripts/release.sh build api|worker|web  build the image locally only (dry run)
#   scripts/release.sh promote [--dry-run]   open (or refresh) the develop → main
#                                     release PR, titled "Release vYYYY.MM.DD.N"
#   scripts/release.sh tag SERVICES [--dry-run]  tag HEAD and publish its GitHub
#                                     Release by hand (SERVICES like api,worker,web);
#                                     a release does this itself once healthy
#
# Every release that passes its health check tags the shipped main commit
# with an annotated date version vYYYY.MM.DD.N (UTC day, N from 1), pushes the
# tag and publishes a GitHub Release whose notes list the PRs since the
# previous tag. A commit already tagged is not tagged again. Tags never move:
# rollback leaves them alone. The rules: docs/BRANCHING_AND_RELEASE.md.
#
# The worker (and `all`) is refused during the nightly pipeline, UTC
# 03:30–08:30: restarting it kills the day's fetch. There is no override.
#
# There is no registry: images are cross-built for amd64 here and piped to the
# server over ssh. The image being replaced is kept as <image>:prev for rollback.
#
# Only merged code ships: HEAD must be clean and contained in origin/main.
# Override for an emergency with RELEASE_ALLOW_UNMERGED=1.
#
# See docs/DEPLOY.md for the full runbook.
set -euo pipefail

HOST="${DEPLOY_HOST:-root@24.199.88.38}"
KEY="${DEPLOY_SSH_KEY:-$HOME/.ssh/cloud_no_pw}"
REMOTE_DIR=/opt/ghta
API_HEALTH=https://api.starrank.dev/health
WEB_HEALTH=https://starrank.dev/en

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

say() { printf '▸ %s\n' "$*"; }
die() { printf '✗ %s\n' "$*" >&2; exit 1; }

# --dry-run (promote, tag): print what would be pushed or created instead.
DRY_RUN=
args=()
for a in "$@"; do
	if [ "$a" = --dry-run ]; then DRY_RUN=1; else args+=("$a"); fi
done
set -- "${args[@]+"${args[@]}"}"
case "${1:-}" in
	promote | tag | "") ;;
	*) [ -z "$DRY_RUN" ] || die "--dry-run is for promote and tag only; for a build without shipping use: scripts/release.sh build <service>" ;;
esac

# Runs a command that changes something on GitHub, or prints it on a dry run.
publish() {
	if [ -n "$DRY_RUN" ]; then
		printf '  [dry-run] %s\n' "$*"
		return 0
	fi
	"$@"
}

# The testable half of this script (versions, PR lists, notes): cmd/release.
# Built once up front by the commands that use it: relcli runs in pipelines
# and command substitutions, where a lazily set variable would not survive.
RELCLI_DIR=
build_relcli() {
	RELCLI_DIR="$(mktemp -d)"
	trap 'rm -rf "$RELCLI_DIR"' EXIT
	go build -o "$RELCLI_DIR/release" ./cmd/release || die "cannot build cmd/release"
}
relcli() { "$RELCLI_DIR/release" "$@"; }

SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=15)
[ -f "$KEY" ] && SSH_OPTS+=(-i "$KEY" -o IdentitiesOnly=yes)
remote() { ssh "${SSH_OPTS[@]}" "$HOST" "$@"; }

image_of() {
	case "$1" in
		api | worker) echo ghta-api ;; # one image, two entrypoints
		web) echo ghta-web ;;
		*) die "unknown service: $1 (api|worker|web)" ;;
	esac
}

health_of() {
	case "$1" in
		api) echo "$API_HEALTH" ;;
		web) echo "$WEB_HEALTH" ;;
	esac
}

# ---- preflight -------------------------------------------------------------

require_releasable_tree() {
	[ -z "$(git status --porcelain)" ] || die "working tree is dirty — commit or stash first"
	if [ "${RELEASE_ALLOW_UNMERGED:-}" = 1 ]; then
		say "RELEASE_ALLOW_UNMERGED=1 — shipping $(git rev-parse --short HEAD) without checking main"
		return
	fi
	git fetch -q origin main
	git merge-base --is-ancestor HEAD origin/main ||
		die "HEAD $(git rev-parse --short HEAD) is not merged into origin/main — merge the PR first"
}

# The nightly pipeline runs in the worker from UTC 03:30 and is done by 08:30;
# a restart in between kills it and the day's remaining fetch is lost.
require_outside_pipeline_window() {
	if relcli window; then
		die "it is $(date -u +%H:%M) UTC, inside the nightly pipeline window (UTC 03:30–08:30): restarting the worker now kills the day's fetch. Release after 08:30 UTC."
	fi
}

# Checked again right before the worker restarts: building and shipping take
# minutes, and a release started at 03:25 must not restart it at 03:35.
guard_worker_restart() {
	relcli window || return 0
	die "it is $(date -u +%H:%M) UTC: the nightly pipeline window (03:30–08:30) began while this release was running, so the worker was not restarted. $1 Ship the worker after 08:30 UTC: scripts/release.sh worker"
}

require_docker() {
	docker info >/dev/null 2>&1 && return
	if [ -d /Applications/OrbStack.app ]; then
		say "starting OrbStack"
		open -ga OrbStack
		for _ in $(seq 1 30); do
			docker info >/dev/null 2>&1 && return
			sleep 2
		done
	fi
	die "docker daemon is not running"
}

require_server() {
	remote true || die "cannot ssh to $HOST"
}

# A new image against a stale compose file (or the reverse) is how env vars and
# network aliases go missing, so refuse to restart until they agree.
require_compose_in_sync() {
	remote "cat $REMOTE_DIR/docker-compose.yml" | diff -q - deploy/docker-compose.yml >/dev/null ||
		die "server compose differs from deploy/docker-compose.yml — run: scripts/release.sh compose"
}

# ---- steps -----------------------------------------------------------------

wait_healthy() {
	local url="$1" code=000
	say "waiting for $url"
	for _ in $(seq 1 30); do
		code="$(curl -s -o /dev/null -m 20 -w '%{http_code}' "$url" || true)"
		[ "$code" = 200 ] && { say "$url → 200"; return 0; }
		sleep 3
	done
	return 1
}

# The running container must be the image we just loaded, not a leftover.
require_running_latest() {
	local svc="$1" image
	image="$(image_of "$svc")"
	remote "test \"\$(docker inspect --format '{{.Image}}' ghta-$svc-1)\" = \"\$(docker image inspect --format '{{.Id}}' $image:latest)\"" ||
		die "$svc container is not running $image:latest"
}

build() {
	local svc="$1" image rev
	image="$(image_of "$svc")"
	rev="$(git rev-parse HEAD)"
	say "building $image @ $(git rev-parse --short HEAD) for linux/amd64"
	local common=(--platform linux/amd64 -t "$image:latest" --load
		--label "org.opencontainers.image.revision=$rev")
	if [ "$svc" != web ]; then
		docker buildx build "${common[@]}" .
	else
		# shellcheck disable=SC1091
		[ -f deploy/release.env ] && . deploy/release.env
		[ -n "${NEXT_PUBLIC_GA_ID:-}" ] || say "NEXT_PUBLIC_GA_ID is empty — this build ships without analytics"
		# API_URL is the public API on purpose: `next build` fetches real data
		# for the SSG pages and the container-internal address doesn't exist yet.
		docker buildx build "${common[@]}" \
			--build-arg API_URL=https://api.starrank.dev \
			--build-arg NEXT_PUBLIC_SITE_URL=https://starrank.dev \
			--build-arg NEXT_PUBLIC_GA_ID="${NEXT_PUBLIC_GA_ID:-}" \
			-f web/Dockerfile web/
	fi
}

ship() {
	local svc="$1" image
	image="$(image_of "$svc")"
	# api and worker share an image. Shipping the commit that is already there
	# would move :prev onto it and lose the real previous image for rollback.
	# Compared by the revision label: image ids differ between image stores.
	local rev_label='{{index .Config.Labels "org.opencontainers.image.revision"}}'
	if [ "$(remote "docker image inspect --format '$rev_label' $image:latest 2>/dev/null || true")" = "$(git rev-parse HEAD)" ]; then
		say "$image on $HOST is already built from this commit — not shipping again"
		return
	fi
	say "keeping the live image as $image:prev"
	remote "docker image inspect $image:latest >/dev/null 2>&1 && docker tag $image:latest $image:prev || true"
	say "shipping $image to $HOST"
	docker save "$image:latest" | gzip -1 | remote 'gunzip | docker load'
}

restart() {
	local svc="$1"
	say "restarting $svc"
	remote "cd $REMOTE_DIR && docker compose up -d $svc"
}

# The worker serves nothing, so "healthy" is: still running a little after the
# start, without having been restarted by docker in between.
wait_worker_running() {
	sleep 10
	remote "test \"\$(docker inspect --format '{{.State.Status}} {{.RestartCount}}' ghta-worker-1)\" = 'running 0'" || return 1
	remote "docker logs --tail 5 ghta-worker-1 2>&1" || true
}

verify() {
	local svc="$1"
	require_running_latest "$svc"
	if [ "$svc" = worker ]; then
		wait_worker_running ||
			die "worker is not staying up — see: docker logs --tail 50 ghta-worker-1; roll back with: scripts/release.sh rollback api"
		return
	fi
	wait_healthy "$(health_of "$svc")" ||
		die "$svc did not come back healthy — roll back with: scripts/release.sh rollback $svc"
}

record() {
	local what="$1"
	printf '%s %s %s %s %s\n' "$(date -u +%FT%TZ)" "$what" "${RELEASE_TAG:--}" "$(git rev-parse --short HEAD)" "$(git log -1 --format=%s)" |
		remote "cat >> $REMOTE_DIR/releases.log; docker image prune -f >/dev/null"
}

release() {
	local svc="$1"
	if [ "$svc" != web ]; then
		say "go vet + go test"
		go vet ./...
		go test ./... -count=1 >/dev/null
	fi
	build "$svc"
	require_compose_in_sync
	ship "$svc"
	[ "$svc" != worker ] || guard_worker_restart "Its image is on the server but not running."
	restart "$svc"
	verify "$svc"
	record "$svc"
	say "$svc released @ $(git rev-parse --short HEAD)"
}

# ---- versions, tags and the release PR --------------------------------------

# Brings in the tags pushed from other checkouts, so versions are counted from
# all of them. Call it at the top level, not inside $(...): a failure there
# would not stop the script.
fetch_tags() {
	git fetch -q --tags origin || die "cannot fetch tags from origin"
}

remote_has_tag() {
	git ls-remote --exit-code --tags origin "refs/tags/$1" >/dev/null
}

# The date version already on HEAD, if any.
head_tag() {
	git tag -l 'v*' --points-at HEAD | relcli latest 2>/dev/null || true
}

# Decides the tag this release will get, before anything ships, so every
# releases.log line of the run carries it. HEAD outside origin/main (an
# RELEASE_ALLOW_UNMERGED emergency) gets none: only main commits are tagged.
plan_tag() {
	RELEASE_TAG=
	TAG_EXISTS=
	git fetch -q origin main
	if ! git merge-base --is-ancestor HEAD origin/main; then
		say "HEAD is not in origin/main — this release will not be tagged"
		return
	fi
	local existing
	fetch_tags
	existing="$(head_tag)"
	if [ -n "$existing" ]; then
		RELEASE_TAG="$existing" TAG_EXISTS=1
		say "HEAD is already tagged $existing — it will not be tagged again"
		return
	fi
	RELEASE_TAG="$(git tag -l 'v*' | relcli next-version)" || die "cannot work out the next version"
	say "this release will be tagged $RELEASE_TAG"
}

# Release notes for HEAD: the PRs since the previous date version it contains.
# With no previous one (the first tag) the newest 30 PR merges stand in.
release_notes() {
	local services="$1" prev
	# The tag being published is excluded: when a cut-short run is finished,
	# it is already on HEAD.
	prev="$(git tag -l 'v*' --merged HEAD | grep -vxF "$RELEASE_TAG" | relcli latest 2>/dev/null || true)"
	# Runs inside $(...), where errexit is off: every step returns its failure.
	if [ -n "$prev" ]; then
		git log --merges --format='%s%x1f%b%x1e' "$prev..HEAD" |
			relcli notes -version "$RELEASE_TAG" -services "$services" || return 1
		printf '\n上一个版本：%s\n' "$prev"
	else
		git log --merges --format='%s%x1f%b%x1e' -n 200 HEAD |
			relcli notes -version "$RELEASE_TAG" -services "$services" -limit 30 || return 1
		printf '\n第一个日期版本：只列最近 30 个 PR。\n'
	fi
}

# Tags HEAD as $RELEASE_TAG, pushes the tag and publishes the GitHub Release.
# Called only after every service of the run passed its health check. Each
# step that is already done is skipped, so a run cut short (the push or the
# Release failed) is finished by `scripts/release.sh tag`; a commit fully
# released before only gets a line in its notes saying what shipped again.
tag_release() {
	local services="$1" notes
	[ -n "${RELEASE_TAG:-}" ] || return 0
	local fresh=1
	if [ -n "${TAG_EXISTS:-}" ]; then
		fresh=
	else
		say "tagging $(git rev-parse --short HEAD) as $RELEASE_TAG"
		if [ -n "$DRY_RUN" ]; then
			printf '  [dry-run] git tag -a %s -m "StarRank %s (%s)" %s\n' "$RELEASE_TAG" "$RELEASE_TAG" "$services" "$(git rev-parse --short HEAD)"
		else
			git tag -a "$RELEASE_TAG" -m "StarRank $RELEASE_TAG ($services)" HEAD
		fi
	fi
	if [ -n "$fresh" ] || ! remote_has_tag "$RELEASE_TAG"; then
		publish git push origin "refs/tags/$RELEASE_TAG"
	fi
	if [ -z "$fresh" ] && gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
		local body
		body="$(gh release view "$RELEASE_TAG" --json body --jq .body)" || die "cannot read the $RELEASE_TAG release notes"
		say "adding the services shipped now to the $RELEASE_TAG release notes"
		publish gh release edit "$RELEASE_TAG" --notes "$(printf '%s\n\n%s 再次发布：%s。\n' "$body" "$(date -u +'%F %H:%M UTC')" "${services//,/、}")"
	else
		notes="$(release_notes "$services")" || die "cannot build the release notes; the tag is in place — finish with: scripts/release.sh tag $services"
		if [ -n "$DRY_RUN" ]; then
			printf '  [dry-run] gh release create %s --verify-tag --latest --title "StarRank %s" --notes <<\n' "$RELEASE_TAG" "$RELEASE_TAG"
			printf '%s\n' "$notes" | sed 's/^/    | /'
		else
			gh release create "$RELEASE_TAG" --verify-tag --latest --title "StarRank $RELEASE_TAG" --notes "$notes" ||
				die "the GitHub Release failed; finish with: scripts/release.sh tag $services"
		fi
	fi
	if [ -n "$DRY_RUN" ]; then say "dry run: nothing was tagged or published"; else say "released as $RELEASE_TAG"; fi
}

# Opens the develop → main release PR, or refreshes the open one: its title
# carries the version the release is expected to get, its body the PRs that
# merging it brings to main.
promote() {
	git fetch -q origin develop main
	[ -n "$(git rev-list -n 1 origin/main..origin/develop)" ] ||
		die "develop has nothing that main lacks — nothing to promote"
	local version body existing
	fetch_tags
	version="$(git tag -l 'v*' | relcli next-version)" || die "cannot work out the next version"
	body="$(git log --merges --format='%s%x1f%b%x1e' origin/main..origin/develop | relcli promote-body -version "$version")" ||
		die "cannot build the release PR body"
	existing="$(gh pr list --base main --head develop --state open --json number --jq '.[0].number // empty')" ||
		die "cannot list the open release PRs"
	local action=(gh pr create --base main --head develop)
	if [ -n "$existing" ]; then
		say "refreshing release PR #$existing as Release $version"
		action=(gh pr edit "$existing")
	else
		say "opening the release PR: Release $version"
	fi
	if [ -n "$DRY_RUN" ]; then
		printf '  [dry-run] %s --title "Release %s" --body <<\n' "${action[*]}" "$version"
		printf '%s\n' "$body" | sed 's/^/    | /'
	else
		"${action[@]}" --title "Release $version" --body "$body"
	fi
}

# ---- commands --------------------------------------------------------------

cmd="${1:-}"
case "$cmd" in
	api | web | worker | all | promote | tag) build_relcli ;;
esac
case "$cmd" in
	api | web | worker)
		[ "$cmd" != worker ] || require_outside_pipeline_window
		require_releasable_tree; require_docker; require_server
		plan_tag
		release "$cmd"
		tag_release "$cmd"
		;;
	all)
		require_outside_pipeline_window
		require_releasable_tree; require_docker; require_server
		plan_tag
		release api
		# Same image as the api just shipped: only the container needs replacing.
		guard_worker_restart "The api is live on the new image; web was not shipped either — run scripts/release.sh web too. Nothing was tagged."
		restart worker
		verify worker
		record worker
		release web
		tag_release api,worker,web
		;;
	promote)
		promote
		;;
	tag)
		services="${2:-}"
		[ -n "$services" ] || die "usage: scripts/release.sh tag api,worker,web [--dry-run]"
		for s in ${services//,/ }; do image_of "$s" >/dev/null; done
		# A dry run creates nothing, so it may run from a scratch checkout.
		[ -n "$DRY_RUN" ] || [ -z "$(git status --porcelain)" ] || die "working tree is dirty — commit or stash first"
		plan_tag
		[ -n "$RELEASE_TAG" ] || die "only commits in origin/main are tagged"
		tag_release "$services"
		;;
	build)
		# Build only — nothing leaves this machine. Useful as a dry run.
		image_of "${2:-}" >/dev/null
		require_releasable_tree; require_docker
		build "$2"
		;;
	compose)
		require_releasable_tree; require_server
		say "syncing docker-compose.yml"
		remote "cp -p $REMOTE_DIR/docker-compose.yml $REMOTE_DIR/docker-compose.yml.prev"
		remote "cat > $REMOTE_DIR/docker-compose.yml" <deploy/docker-compose.yml
		remote "cd $REMOTE_DIR && docker compose up -d"
		wait_healthy "$API_HEALTH" && wait_healthy "$WEB_HEALTH" ||
			die "unhealthy after compose sync — previous file is $REMOTE_DIR/docker-compose.yml.prev"
		record compose
		;;
	site)
		require_releasable_tree; require_server
		say "publishing deploy/starrank.caddy"
		remote /opt/edge/bin/apply-site starrank <deploy/starrank.caddy
		wait_healthy "$API_HEALTH" && wait_healthy "$WEB_HEALTH" || die "unhealthy after site publish"
		record site
		;;
	rollback)
		svc="${2:-}"
		[ "$svc" != worker ] || die "the worker shares the api image — use: scripts/release.sh rollback api"
		image="$(image_of "$svc")"
		require_server
		remote "docker image inspect $image:prev >/dev/null 2>&1" || die "no $image:prev on the server"
		say "rolling $svc back to $image:prev"
		# Swap, so rolling back twice returns to where we started.
		remote "docker tag $image:latest $image:rollback-tmp && docker tag $image:prev $image:latest && docker tag $image:rollback-tmp $image:prev && docker rmi $image:rollback-tmp >/dev/null"
		restart "$svc"
		verify "$svc"
		if [ "$svc" = api ]; then
			restart worker
			verify worker
		fi
		# Tags stay where they are: the log says what is live after a rollback.
		printf '%s rollback-%s -\n' "$(date -u +%FT%TZ)" "$svc" | remote "cat >> $REMOTE_DIR/releases.log"
		;;
	status)
		require_server
		remote "cd $REMOTE_DIR && docker compose ps --format 'table {{.Service}}\t{{.Status}}'
			for s in api worker web; do
				r=\$(docker inspect --format '{{index .Config.Labels \"org.opencontainers.image.revision\"}}' ghta-\$s-1 2>/dev/null || true)
				echo \"\$s revision: \${r:-unknown}\"
			done
			echo '--- last releases'; tail -n 8 $REMOTE_DIR/releases.log 2>/dev/null || echo '(none recorded)'"
		;;
	*)
		sed -n '2,38p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
		exit 2
		;;
esac
