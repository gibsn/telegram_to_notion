package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	notionapi "github.com/gibsn/telegram_to_notion/internal/notion"
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

	notion := notionapi.NewNotion(notionToken)

	p := requestprocessor.NewRequestProcessor(notion, notionDBID, bot)
	p.SetDebug(true)

	var reply string

	req := notionapi.NewCreateTaskRequest()
	req.TaskName = "test_task"
	req.Description = "test_description"
	req.Assignees = []string{"@gibsn"}

	url, err := p.CreateTask(req)
	if err != nil {
		log.Printf("error: %s", err)
		reply = err.Error()
	} else {
		reply = fmt.Sprintf(
			"Task has been successfully created and assigned to %s:\n%s",
			strings.Join([]string{"@gibsn"}, ","),
			url,
		)
	}

	msg := tgbotapi.NewMessage(51451990, reply)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Could not send message to Telegram: %v", err)
	}
}
