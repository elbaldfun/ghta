#!/usr/bin/env bash
# Release starrank.dev to the production droplet.
#
#   scripts/release.sh api            build + ship + restart the Go API
#   scripts/release.sh web            build + ship + restart the Next.js frontend
#   scripts/release.sh all            api, then web (backend first: the frontend
#                                     may send params an older API answers 400 to)
#   scripts/release.sh compose        sync deploy/docker-compose.yml, recreate what changed
#   scripts/release.sh site           publish deploy/starrank.caddy to the shared Caddy
#   scripts/release.sh rollback api|web   put the previous image back
#   scripts/release.sh status         what is live right now
#   scripts/release.sh build api|web  build the image locally only (dry run)
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

SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=15)
[ -f "$KEY" ] && SSH_OPTS+=(-i "$KEY" -o IdentitiesOnly=yes)
remote() { ssh "${SSH_OPTS[@]}" "$HOST" "$@"; }

image_of() {
	case "$1" in
		api) echo ghta-api ;;
		web) echo ghta-web ;;
		*) die "unknown service: $1 (api|web)" ;;
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
	if [ "$svc" = api ]; then
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

verify() {
	local svc="$1"
	require_running_latest "$svc"
	wait_healthy "$(health_of "$svc")" ||
		die "$svc did not come back healthy — roll back with: scripts/release.sh rollback $svc"
}

record() {
	local what="$1"
	printf '%s %s %s %s\n' "$(date -u +%FT%TZ)" "$what" "$(git rev-parse --short HEAD)" "$(git log -1 --format=%s)" |
		remote "cat >> $REMOTE_DIR/releases.log; docker image prune -f >/dev/null"
}

release() {
	local svc="$1"
	if [ "$svc" = api ]; then
		say "go vet + go test"
		go vet ./...
		go test ./... -count=1 >/dev/null
	fi
	build "$svc"
	require_compose_in_sync
	ship "$svc"
	restart "$svc"
	verify "$svc"
	record "$svc"
	say "$svc released @ $(git rev-parse --short HEAD)"
}

# ---- commands --------------------------------------------------------------

cmd="${1:-}"
case "$cmd" in
	api | web)
		require_releasable_tree; require_docker; require_server
		release "$cmd"
		;;
	all)
		require_releasable_tree; require_docker; require_server
		release api
		release web
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
		image="$(image_of "$svc")"
		require_server
		remote "docker image inspect $image:prev >/dev/null 2>&1" || die "no $image:prev on the server"
		say "rolling $svc back to $image:prev"
		# Swap, so rolling back twice returns to where we started.
		remote "docker tag $image:latest $image:rollback-tmp && docker tag $image:prev $image:latest && docker tag $image:rollback-tmp $image:prev && docker rmi $image:rollback-tmp >/dev/null"
		restart "$svc"
		verify "$svc"
		printf '%s rollback-%s\n' "$(date -u +%FT%TZ)" "$svc" | remote "cat >> $REMOTE_DIR/releases.log"
		;;
	status)
		require_server
		remote "cd $REMOTE_DIR && docker compose ps --format 'table {{.Service}}\t{{.Status}}'
			for s in api web; do
				r=\$(docker inspect --format '{{index .Config.Labels \"org.opencontainers.image.revision\"}}' ghta-\$s-1 2>/dev/null || true)
				echo \"\$s revision: \${r:-unknown}\"
			done
			echo '--- last releases'; tail -n 8 $REMOTE_DIR/releases.log 2>/dev/null || echo '(none recorded)'"
		;;
	*)
		sed -n '2,13p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
		exit 2
		;;
esac
