package main

import (
	"flag"
	"log"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/requestprocessor"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	var botToken, notionToken, notionDBID string

	flag.StringVar(&botToken, "telegram_token", "", "Telegram Bot Token")
	flag.StringVar(&notionToken, "notion_token", "", "Notion Integration Token")
	flag.StringVar(&notionDBID, "notion_db", "", "Notion Database ID")
	flag.Parse()

	if botToken == "" || notionToken == "" || notionDBID == "" {
		log.Fatal("All parameters (telegram_token, notion_token, notion_db) are required")
	}

	log.Printf("Will connect to Telgram")

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Could not connect to the Telegram API: %v", err)
	}

	log.Printf("Successfully connected to Telegram")

	notion := notion.NewNotion(notionToken)

	p := requestprocessor.NewRequestProcessor(notion, notionDBID, bot)
	p.ProcessRequests()
}
