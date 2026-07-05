#!/usr/bin/env bash
set -euo pipefail

exec /home/ai/telegram_to_notion/bin/telegram_to_notion \
	-telegram_token="${TELEGRAM_TOKEN:?TELEGRAM_TOKEN is required}" \
	-notion_token="${NOTION_TOKEN:?NOTION_TOKEN is required}" \
	-tasks_db="${TASKS_DB:?TASKS_DB is required}" \
	-tweaks_db="${TWEAKS_DB:?TWEAKS_DB is required}" \
	-tweaks_mix_db="${TWEAKS_MIX_DB:?TWEAKS_MIX_DB is required}" \
	-tracks_db="${TRACKS_DB:?TRACKS_DB is required}" \
	-ping_chat_id="${PING_CHAT_ID:-0}" \
	-ping_threshold="${PING_THRESHOLD:-72h}" \
	-ping_st_time="${PING_STARTING_TIME:-09:00}" \
	-ping_end_time="${PING_END_TIME:-23:00}" \
	-ping_period_time="${PING_PERIOD:-6h}" \
	-ping_text="${PING_TEXT:-Hi, what's the estimate?}" \
	-tasks_cache_period="${TASKS_CACHE_PERIOD:-1m}" \
	-tracks_cache_period="${TRACKS_CACHE_PERIOD:-1m}" \
	-debug="${DEBUG:-false}"
