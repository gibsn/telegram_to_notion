package main

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type telegramRequester interface {
	Request(tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

func botCommands() []tgbotapi.BotCommand {
	return []tgbotapi.BotCommand{
		{Command: "task", Description: "Create a task"},
		{Command: "agenda", Description: "Create an agenda item"},
		{Command: "deadline", Description: "Set a task deadline"},
		{Command: "done", Description: "Complete a task"},
		{Command: "tasks", Description: "Show active tasks"},
		{Command: "tracks", Description: "Show tweak tracks"},
		{Command: "tweak", Description: "Create or process a tweak"},
		{Command: "cancel", Description: "Cancel the current action"},
	}
}

func registerBotCommands(bot telegramRequester) error {
	if _, err := bot.Request(tgbotapi.NewSetMyCommands(botCommands()...)); err != nil {
		return fmt.Errorf("set Telegram bot commands: %w", err)
	}

	return nil
}
