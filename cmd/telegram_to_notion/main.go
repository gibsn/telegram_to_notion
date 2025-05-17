package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type RequestProcessor struct {
	notionToken string
	notionDBID  string

	bot          *tgbotapi.BotAPI
	nameResolver *notion.UserResolver
}

func NewRequestProcessor(token, dbid string, bot *tgbotapi.BotAPI) *RequestProcessor {
	p := &RequestProcessor{
		notionToken: token,
		notionDBID:  dbid,
		bot:         bot,
	}

	p.nameResolver = notion.NewUserResolver()

	return p
}

func (p *RequestProcessor) processRequests() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	for update := range p.bot.GetUpdatesChan(u) {
		if update.Message == nil || !strings.HasPrefix(update.Message.Text, "/task") {
			continue
		}

		lines := strings.Split(update.Message.Text, "\n")
		if len(lines) < 2 {
			msg := tgbotapi.NewMessage(
				update.Message.Chat.ID, "Please provide the task's name and an assignee",
			)

			if _, err := p.bot.Send(msg); err != nil {
				log.Printf("Could not send message to Telegram: %v", err)
			}

			continue
		}

		taskName := strings.TrimPrefix(lines[0], "/task ")
		assignee := lines[1]

		description := ""
		if len(lines) > 2 {
			description = strings.Join(lines[2:], "\n")
		}

		var reply string

		url, err := p.createTask(taskName, assignee, description)
		if err != nil {
			log.Printf("error: %s", err)
			reply = err.Error()
		} else {
			reply = fmt.Sprintf("Task has been successfully created:\n%s", url)
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		if _, err := p.bot.Send(msg); err != nil {
			log.Printf("Could not send message to Telegram: %v", err)
		}
	}
}

func (p *RequestProcessor) createTask(taskName, name, description string) (string, error) {
	assignee := p.nameResolver.Resolve(name)
	if assignee == "" {
		return "", fmt.Errorf("unknown assignee %s", name)
	}

	url, err := notion.CreateNotionTask(p.notionToken, p.notionDBID, taskName, assignee, description)
	if err != nil {
		return "", fmt.Errorf("error creating a task in Notion: %w", err)
	}

	return url, nil
}

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

	p := NewRequestProcessor(notionToken, notionDBID, bot)
	p.processRequests()
}
