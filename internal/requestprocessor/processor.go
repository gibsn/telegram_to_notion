package requestprocessor

import (
	"fmt"
	"log"
	"strings"

	"github.com/gibsn/telegram_to_notion/internal/notion"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type RequestProcessor struct {
	notion     *notion.Notion
	notionDBID string

	bot          *tgbotapi.BotAPI
	nameResolver *UserResolver

	allowedToCreate map[string]bool

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
		"@homesick94":   "3c02801c-1a5a-428f-b217-6d53032a21c9",
		"@gibsn":        "7439e2ca-75f8-4024-b170-620ef7ed08b1",
		"@bond_lullaby": "aea80e9c-7a69-4180-8a38-6d274af25f4c",
	}

	return r
}

func (r *UserResolver) Resolve(tgName string) string {
	return r.mapping[strings.ToLower(strings.TrimSpace(tgName))]
}

func (r *UserResolver) ResolveArr(tgNames []string) ([]string, error) {
	resolved := make([]string, 0, len(tgNames))

	for _, tgName := range tgNames {
		resolvedName := r.Resolve(tgName)
		if resolvedName == "" {
			return nil, fmt.Errorf("login unknown: %s", tgName)
		}

		resolved = append(resolved, resolvedName)
	}

	return resolved, nil
}

func NewRequestProcessor(
	notion *notion.Notion, dbid string, bot *tgbotapi.BotAPI,
) *RequestProcessor {
	p := &RequestProcessor{
		notion:     notion,
		notionDBID: dbid,
		bot:        bot,
	}

	p.nameResolver = NewUserResolver()
	p.allowedToCreate = map[string]bool{
		"alexander_zh": true,
		"vomadan":      true,
		"fenyakolles":  true,
		"nikitacmc":    true,
		"homesick94":   true,
		"gibsn":        true,
	}

	return p
}

func (p *RequestProcessor) SetDebug(debug bool) {
	p.debug = debug
}

func (p *RequestProcessor) parseAndValidateTelegramRequest(update tgbotapi.Update) (
	*notion.CreateTaskRequest, error,
) {
	fromUserName := strings.ToLower(update.Message.From.UserName)
	isPrivate := update.Message.Chat.IsPrivate()

	if !p.allowedToCreate[fromUserName] {
		return nil, fmt.Errorf("user %s is not allowed to create tasks", fromUserName)
	}

	req, err := parseTelegramRequestMessage(update.Message.Text, isPrivate)

	// set assignee to the sender if the message came from direct messages
	if isPrivate {
		req.Assignees = []string{"@" + fromUserName}
	}

	return req, err
}

func parseTelegramRequestMessage(text string, isPrivate bool) (
	*notion.CreateTaskRequest, error,
) {
	if !strings.HasPrefix(text, "/task") {
		return nil, fmt.Errorf("unknown command")
	}

	lines := strings.Split(text, "\n")

	if !isPrivate && len(lines) < 2 {
		return nil, fmt.Errorf("please provide the task's name and an assignee")
	}

	req := notion.NewCreateTaskRequest()

	// the first line is the command itself and it has the task's name
	req.TaskName = strings.TrimSpace(strings.TrimPrefix(lines[0], "/task"))
	if len(req.TaskName) == 0 {
		return nil, fmt.Errorf("please provide the task's name")
	}

	// the second line is the assignee if the message came from the public chat. if the
	// message came from direct messages then the assignee is set to the sender
	if len(lines) >= 2 {
		if isPrivate {
			req.Description = strings.Join(lines[1:], "\n")
		} else {
			req.Assignees = strings.Fields(lines[1])
		}
	}

	// the third line is only present in public chats and is optional. it contains
	// description if present
	if len(lines) >= 3 && !isPrivate {
		req.Description = strings.Join(lines[2:], "\n")
	}

	return req, nil
}

func (p *RequestProcessor) ProcessRequests() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	for update := range p.bot.GetUpdatesChan(u) {
		if update.Message == nil {
			continue
		}

		req, err := p.parseAndValidateTelegramRequest(update)
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
	req.NotionDBID = p.notionDBID

	assigneesResolved, err := p.nameResolver.ResolveArr(req.Assignees)
	if err != nil {
		return "", err
	}

	req.Assignees = assigneesResolved

	if p.debug {
		req.Debug = true
	}

	url, err := p.notion.CreateNotionTask(req)
	if err != nil {
		return "", fmt.Errorf("error creating a task in Notion: %w", err)
	}

	return url, nil
}
