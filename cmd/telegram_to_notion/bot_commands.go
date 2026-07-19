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
		{Command: "task", Description: "Создать задачу"},
		{Command: "agenda", Description: "Создать пункт повестки"},
		{Command: "deadline", Description: "Установить дедлайн задачи"},
		{Command: "done", Description: "Завершить задачу"},
		{Command: "tasks", Description: "Показать активные задачи"},
		{Command: "tracks", Description: "Показать треки правок"},
		{Command: "tweak", Description: "Создать или обработать правку"},
	}
}

func registerBotCommands(bot telegramRequester) error {
	if _, err := bot.Request(tgbotapi.NewSetMyCommands(botCommands()...)); err != nil {
		return fmt.Errorf("set Telegram bot commands: %w", err)
	}

	return nil
}
