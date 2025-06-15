package main

import (
	"flag"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	var botToken string

	flag.StringVar(&botToken, "telegram_token", "", "Telegram Bot Token")
	flag.Parse()

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Could not connect to the Telegram API: %v", err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			log.Printf("Chat ID: %d, text: %s", update.Message.Chat.ID, update.Message.Text)
		}
	}
}
