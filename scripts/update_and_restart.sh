#!/usr/bin/env bash
set -euo pipefail

repo_dir="${REPO_DIR:-/home/ai/telegram_to_notion}"
remote="${UPDATE_REMOTE:-origin}"
branch="${UPDATE_BRANCH:-main}"
service="${SERVICE_NAME:-telegram_to_notion.service}"
repo_user="${REPO_USER:-telegram_to_notion}"

as_repo_user() {
	if [[ "$(id -u)" -eq 0 ]]; then
		runuser -u "$repo_user" -- "$@"
		return
	fi

	"$@"
}

cd "$repo_dir"

old_head="$(as_repo_user git rev-parse HEAD)"

as_repo_user git fetch --prune "$remote" "$branch"
remote_head="$(as_repo_user git rev-parse "$remote/$branch")"

if [[ "$old_head" == "$remote_head" ]]; then
	echo "No changes in $remote/$branch"
	exit 0
fi

if ! as_repo_user git diff --quiet || ! as_repo_user git diff --cached --quiet; then
	echo "Working tree is dirty; refusing to auto-update $repo_dir"
	exit 1
fi

as_repo_user git merge --ff-only "$remote/$branch"
as_repo_user make telegram_to_notion

systemctl restart "$service"
echo "Updated $service from $old_head to $remote_head"
