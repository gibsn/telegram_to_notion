#!/usr/bin/env bash
set -euo pipefail

repo_dir="${REPO_DIR:-/home/telegram_to_notion/telegram_to_notion}"
remote="${UPDATE_REMOTE:-origin}"
branch="${UPDATE_BRANCH:-main}"
service="${SERVICE_NAME:-telegram_to_notion.service}"
repo_user="${REPO_USER:-telegram_to_notion}"
telegram_config_dir="${TELEGRAM_CONFIG_DIR:-/home/ai/.config/telegram}"
telegram_token_file="${TELEGRAM_BOT_TOKEN_FILE:-$telegram_config_dir/gibsn_codex_bot_token}"
telegram_chat_id_file="${TELEGRAM_CHAT_ID_FILE:-$telegram_config_dir/gibsn_codex_chat_id}"
notify_token="${CODEX_TELEGRAM_BOT_TOKEN:-${TELEGRAM_BOT_TOKEN:-}}"
notify_chat_id="${CODEX_TELEGRAM_CHAT_ID:-${TELEGRAM_CHAT_ID:-}}"

as_repo_user() {
	if [[ "$(id -u)" -eq 0 ]]; then
		runuser -u "$repo_user" -- "$@"
		return
	fi

	"$@"
}

read_secret_file() {
	local path="$1"
	if [[ -r "$path" ]]; then
		tr -d '\r\n' < "$path"
	fi
}

notify_deploy() {
	if [[ -z "$notify_token" ]]; then
		notify_token="$(read_secret_file "$telegram_token_file")"
	fi
	if [[ -z "$notify_chat_id" ]]; then
		notify_chat_id="$(read_secret_file "$telegram_chat_id_file")"
	fi

	if [[ -z "$notify_token" || -z "$notify_chat_id" ]]; then
		echo "Telegram deploy notification is disabled"
		return
	fi

	local old_short new_short host message
	old_short="$(as_repo_user git rev-parse --short "$old_head")"
	new_short="$(as_repo_user git rev-parse --short "$remote_head")"
	host="$(hostname -f 2>/dev/null || hostname)"
	message="Выкатка ${service} на ${host}: ${old_short} -> ${new_short} (${remote}/${branch})"

	if ! curl -fsS \
		--data-urlencode "chat_id=${notify_chat_id}" \
		--data-urlencode "text=${message}" \
		"https://api.telegram.org/bot${notify_token}/sendMessage" >/dev/null; then
		echo "Could not send Telegram deploy notification"
	fi
}

cd "$repo_dir"

old_head="$(as_repo_user git rev-parse HEAD)"

as_repo_user git fetch --prune "$remote" "$branch"
remote_head="$(as_repo_user git rev-parse "$remote/$branch")"

if [[ "$old_head" == "$remote_head" ]]; then
	echo "No changes in $remote/$branch"
	exit 0
fi

if as_repo_user git merge-base --is-ancestor "$remote_head" "$old_head"; then
	echo "Local HEAD already contains $remote/$branch"
	exit 0
fi

if ! as_repo_user git diff --quiet || ! as_repo_user git diff --cached --quiet; then
	echo "Working tree is dirty; refusing to auto-update $repo_dir"
	exit 1
fi

as_repo_user git merge --ff-only "$remote/$branch"
as_repo_user make telegram_to_notion

systemctl restart "$service"
notify_deploy
echo "Updated $service from $old_head to $remote_head"
