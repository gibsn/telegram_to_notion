package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	notionapi "github.com/gibsn/telegram_to_notion/internal/notion"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	var botToken, notionToken, notionDBID string
	var debug bool

	flag.StringVar(&botToken, "telegram_token", "", "Telegram Bot Token")
	flag.StringVar(&notionToken, "notion_token", "", "Notion Integration Token")
	flag.StringVar(&notionDBID, "notion_db", "", "Notion Database ID")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
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

	var reply string

	req := notionapi.NewCreateTaskRequest()
	req.NotionDBID = notionDBID
	req.TaskName = "test_task_2"
	req.Description = "test_description_2"
	req.Assignees = []string{"7439e2ca-75f8-4024-b170-620ef7ed08b1"}
	req.Debug = debug

	url, err := notion.CreateNotionTask(req)
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
