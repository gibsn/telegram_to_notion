package requestprocessor

import (
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
	nameResolver *UserResolver

	debug bool
}

type UserResolver struct {
	mapping map[string]string
}

func NewUserResolver() *UserResolver {
	r := &UserResolver{}

	r.mapping = map[string]string{
		"@alexander_zh": "9e8f4963-fd1c-4bb5-bdd2-7f29a9a8698a",
		"@vomadan":      "0724b18e-320d-4fce-87f6-95d69b51c2c0",
		"@fenyakolles":  "78694531-146f-4abd-b29b-093278cab708",
		"@nikitacmc":    "e6f7887a-7123-4a83-a5da-ded24467d5e2",
		"@Homesick94":   "3c02801c-1a5a-428f-b217-6d53032a21c9",
		"@bond_lullaby": "aea80e9c-7a69-4180-8a38-6d274af25f4c",
		"@gibsn":        "7439e2ca-75f8-4024-b170-620ef7ed08b1",
	}

	return r
}

func (r *UserResolver) Resolve(tgName string) string {
	return r.mapping[strings.TrimSpace(tgName)]
}

func (r *UserResolver) ResolveArr(tgNames []string) ([]string, error) {
	resolved := make([]string, len(tgNames), len(tgNames))

	for i, tgName := range tgNames {
		resolvedName := r.Resolve(tgName)
		if resolvedName == "" {
			return nil, fmt.Errorf("login unknown: %s", tgName)
		}

		resolved[i] = resolvedName
	}

	return resolved, nil
}

func NewRequestProcessor(token, dbid string, bot *tgbotapi.BotAPI) *RequestProcessor {
	p := &RequestProcessor{
		notionToken: token,
		notionDBID:  dbid,
		bot:         bot,
	}

	p.nameResolver = NewUserResolver()

	return p
}

func (p *RequestProcessor) SetDebug(debug bool) {
	p.debug = debug
}

func parseTelegramRequest(update tgbotapi.Update) (
	*notion.CreateTaskRequest, error,
) {
	req, err := parseTelegramRequestMessage(update.Message.Text)

	return req, err
}

func parseTelegramRequestMessage(text string) (
	*notion.CreateTaskRequest, error,
) {
	if !strings.HasPrefix(text, "/task") {
		return nil, fmt.Errorf("unknown command")
	}

	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("Please provide the task's name and an assignee")
	}

	taskName := strings.TrimPrefix(lines[0], "/task ")

	description := ""
	if len(lines) > 2 {
		description = strings.Join(lines[2:], "\n")
	}

	req := notion.NewCreateTaskRequest()
	req.TaskName = taskName
	req.Description = description
	req.Assignees = strings.Fields(lines[1])

	return req, nil
}

func (p *RequestProcessor) ProcessRequests() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	for update := range p.bot.GetUpdatesChan(u) {
		if update.Message == nil {
			continue
		}

		req, err := parseTelegramRequest(update)
		if err != nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())

			if _, err := p.bot.Send(msg); err != nil {
				log.Printf("Could not send message to Telegram: %v", err)
			}

			continue
		}

		var reply string

		url, err := p.CreateTask(req)
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

func (p *RequestProcessor) CreateTask(req *notion.CreateTaskRequest) (string, error) {
	req.NotionToken = p.notionToken
	req.NotionDBID = p.notionDBID

	assigneesResolved, err := p.nameResolver.ResolveArr(req.Assignees)
	if err != nil {
		return "", err
	}

	req.Assignees = assigneesResolved

	if p.debug {
		req.Debug = true
	}

	url, err := notion.CreateNotionTask(req)
	if err != nil {
		return "", fmt.Errorf("error creating a task in Notion: %w", err)
	}

	return url, nil
}
