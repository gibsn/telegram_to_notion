package requestprocessor

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/taskscache"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	errInvalidCommand = errors.New("invalid command")
	errUnknownCommand = errors.New("unknown command")
)

type RequestProcessor struct {
	notion     *notion.Notion
	notionDBID string

	bot          *tgbotapi.BotAPI
	nameResolver *UserResolver
	tasksCache   *taskscache.Cache

	allowedToCreate map[string]bool

	taskLinkParser *regexp.Regexp

	debug bool
}

func NewRequestProcessor(
	notion *notion.Notion, dbid string, bot *tgbotapi.BotAPI,
) *RequestProcessor {
	p := &RequestProcessor{
		notion:     notion,
		notionDBID: dbid,
		bot:        bot,
	}

	p.taskLinkParser = regexp.MustCompile(`https://www.notion.so/[\w\d\-]+`)

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

func (p *RequestProcessor) SetTasksCache(cache *taskscache.Cache) {
	p.tasksCache = cache
}

type commandCommon struct {
	command           string
	restOfMessage     string
	repliedToText     string
	repliedToEntities []tgbotapi.MessageEntity
	fromUserName      string
	isPrivate         bool
}

func (p *RequestProcessor) parseAndValidateTelegramRequest(update tgbotapi.Update) (
	commandCommon, error,
) {
	fromUserName := strings.ToLower(update.Message.From.UserName)

	if !p.allowedToCreate[fromUserName] {
		return commandCommon{}, fmt.Errorf("user %s is not allowed to send commands", fromUserName)
	}

	command := extractCommand(update.Message.Text)

	if update.Message.ReplyToMessage != nil {
		command.repliedToText = update.Message.ReplyToMessage.Text
		command.repliedToEntities = update.Message.ReplyToMessage.Entities
	}
	command.isPrivate = update.Message.Chat.IsPrivate()
	command.fromUserName = fromUserName

	return command, nil
}

func extractCommand(text string) commandCommon {
	firstLine := strings.SplitN(text, "\n", 2)[0]
	command := strings.SplitN(firstLine, " ", 2)[0]

	var restOfMessage string
	if commandAndRest := strings.SplitN(text, " ", 2); len(commandAndRest) > 1 {
		restOfMessage = commandAndRest[1]
	}

	return commandCommon{
		command:       command,
		restOfMessage: restOfMessage,
	}
}

func parseTaskCommand(message commandCommon) (
	*notion.CreateTaskRequest, error,
) {
	lines := strings.Split(message.restOfMessage, "\n")

	if !message.isPrivate && len(lines) < 2 {
		return nil, fmt.Errorf("please provide the task's name and an assignee")
	}

	req := notion.NewCreateTaskRequest()

	// the first line is the command itself and it has the task's name
	req.TaskName = strings.TrimSpace(lines[0])
	if len(req.TaskName) == 0 {
		return nil, fmt.Errorf("please provide the task's name")
	}

	// the second line is the assignee if the message came from the public chat. if the
	// message came from direct messages then the assignee is set to the sender
	if len(lines) >= 2 {
		if message.isPrivate {
			req.Description = strings.Join(lines[1:], "\n")
		} else {
			req.Assignees = strings.Fields(lines[1])
		}
	}

	// the third line is only present in public chats and is optional. it contains
	// description if present
	if len(lines) >= 3 && !message.isPrivate {
		req.Description = strings.Join(lines[2:], "\n")
	}

	// set assignee to the sender if the message came from direct messages
	if message.isPrivate {
		req.Assignees = []string{"@" + message.fromUserName}
	}

	return req, nil
}

func (p *RequestProcessor) extractTaskLink(message commandCommon) string {
	for _, entity := range message.repliedToEntities {
		if entity.Type == "text_link" {
			if p.taskLinkParser.MatchString(entity.URL) {
				return entity.URL
			}
		}
	}

	// if not found in entities, try to find in the text
	match := p.taskLinkParser.FindString(message.repliedToText)

	return match
}

func (p *RequestProcessor) parseSetDeadlineCommand(message commandCommon) (
	*notion.SetDeadlineRequest, error,
) {
	req := &notion.SetDeadlineRequest{}

	if message.repliedToText == "" {
		return nil, fmt.Errorf("command is not a reply to any message")
	}

	taskLink := p.extractTaskLink(message)
	if taskLink == "" {
		return nil, fmt.Errorf("command is not a reply to a task")
	}

	req.TaskLink = taskLink

	deadlineStr := strings.TrimSpace(message.restOfMessage)
	deadlineParsed, err := time.Parse("2006-01-02", deadlineStr)
	if err != nil {
		return nil, fmt.Errorf("invalid deadline %s", deadlineStr)
	}

	req.Deadline = deadlineParsed

	return req, err
}

func (p *RequestProcessor) parseDoneCommand(message commandCommon) (
	*notion.SetStatusRequest, error,
) {
	req := &notion.SetStatusRequest{}

	if message.repliedToText == "" {
		return nil, fmt.Errorf("command is not a reply to any message")
	}

	taskLink := p.extractTaskLink(message)
	if taskLink == "" {
		return nil, fmt.Errorf("command is not a reply to a task")
	}

	req.TaskLink = taskLink
	req.Status = notion.StatusDone

	return req, nil
}

func (p *RequestProcessor) parseTasksCommand(message commandCommon) (
	string, error,
) {
	userID := p.nameResolver.TgToNotion("@" + message.fromUserName)
	if userID == "" {
		return "", fmt.Errorf("user %s is not found in the system", message.fromUserName)
	}

	return userID, nil
}

type commandHandler func(commandCommon) (string, error)

func (p *RequestProcessor) ProcessRequests() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	for update := range p.bot.GetUpdatesChan(u) {
		if update.Message == nil {
			continue
		}

		reply, err := p.processRequest(update)
		if err != nil {
			log.Printf("Got an invalid message from %s: %v", update.Message.From.UserName, err)
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		msg.ParseMode = "HTML"

		if _, err := p.bot.Send(msg); err != nil {
			log.Printf("Could not send message to Telegram: %v", err)
		}
	}
}

func (p *RequestProcessor) processRequest(update tgbotapi.Update) (string, error) {
	message, err := p.parseAndValidateTelegramRequest(update)
	if err != nil {
		return "", err
	}

	var reply string

	switch message.command {
	case "/task":
		reply, err = withErrorReply(message, p.processTask)
	case "/deadline":
		reply, err = withErrorReply(message, p.processDeadline)
	case "/done":
		reply, err = withErrorReply(message, p.processDone)
	case "/tasks":
		reply, err = withErrorReply(message, p.processTasks)
	default:
		err = errUnknownCommand
		reply = "🖕🖕🖕"
	}

	return reply, err
}

func withErrorReply(message commandCommon, cb commandHandler) (string, error) {
	reply, err := cb(message)
	if err == nil {
		return reply, nil
	}

	if !errors.Is(err, errInvalidCommand) {
		return err.Error(), err
	}

	switch message.command {
	case "/task":
		reply = fmt.Sprintf(
			"%s\n\nUsage:\n/task $task_name\n$assignee1 $assignee2 ...\n$task_description (optional)",
			err.Error(),
		)
	case "/deadline":
		reply = fmt.Sprintf(
			"%s\n\nMust be a reply to a message with task link \nUsage:\n/deadline YYYY-MM-DD",
			err.Error(),
		)
	case "/done":
		reply = fmt.Sprintf(
			"%s\n\nMust be a reply to a message with task link \nUsage:\n/done",
			err.Error(),
		)
	case "/tasks":
		reply = fmt.Sprintf(
			"%s\n\nUsage:\n/tasks",
			err.Error(),
		)
	}

	return reply, err
}

func (p *RequestProcessor) processTask(message commandCommon) (string, error) {
	req, err := parseTaskCommand(message)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errInvalidCommand, err)
	}

	req.NotionDBID = p.notionDBID

	assigneesResolved, err := p.nameResolver.ResolveArr(req.Assignees)
	if err != nil {
		return "", err
	}

	reqCopy := *req
	reqCopy.Assignees = assigneesResolved

	if p.debug {
		req.Debug = true
	}

	url, err := p.notion.CreateNotionTask(&reqCopy)
	if err != nil {
		return "", fmt.Errorf("error creating a task in Notion: %w", err)
	}

	reply := fmt.Sprintf(
		"Task has been successfully created and assigned to %s:\n%s",
		strings.Join(req.Assignees, ", "),
		url,
	)

	return reply, nil
}

func (p *RequestProcessor) processDeadline(message commandCommon) (string, error) {
	req, err := p.parseSetDeadlineCommand(message)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errInvalidCommand, err)
	}

	if p.debug {
		req.Debug = true
	}

	if err := p.notion.SetDeadline(req); err != nil {
		return "", fmt.Errorf("could not set deadline to %s: %w", req.Deadline.Format("2006-01-02"), err)
	}

	reply := fmt.Sprintf(
		"Deadline has been successfully set to %s",
		req.Deadline.Format("2006-01-02"),
	)

	return reply, nil
}

func (p *RequestProcessor) processDone(message commandCommon) (string, error) {
	req, err := p.parseDoneCommand(message)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errInvalidCommand, err)
	}

	if p.debug {
		req.Debug = true
	}

	if err := p.notion.SetStatus(req); err != nil {
		return "", fmt.Errorf("could not set status to done: %w", err)
	}

	reply := "Task has been successfully marked as Done"

	return reply, nil
}

func (p *RequestProcessor) processTasks(message commandCommon) (string, error) {
	userID, err := p.parseTasksCommand(message)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errInvalidCommand, err)
	}

	if p.tasksCache == nil {
		return "", fmt.Errorf("tasks cache is not initialized")
	}

	tasks, err := p.tasksCache.GetTasksForUser(userID)
	if err != nil {
		return "", fmt.Errorf("error loading tasks: %w", err)
	}

	if len(tasks) == 0 {
		return "No tasks found for you", nil
	}

	var reply strings.Builder
	reply.WriteString("Your tasks:\n\n")

	for i, task := range tasks {
		reply.WriteString(fmt.Sprintf("%d. <a href=\"%s\">%s</a>", i+1, task.Link, task.Title))

		if !task.Deadline.IsZero() {
			reply.WriteString(fmt.Sprintf(" (Deadline: %s)", task.Deadline.Format("2006-01-02")))
		}

		reply.WriteString("\n")
	}

	return reply.String(), nil
}
