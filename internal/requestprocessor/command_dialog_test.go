package requestprocessor

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandsWithoutArgumentsStartInputDialog(t *testing.T) {
	tests := []struct {
		name              string
		text              string
		wantCommand       string
		chatType          string
		repliedToText     string
		wantPrompt        string
		wantPlaceholder   string
		wantReplyContext  string
		wantSelectiveMode bool
	}{
		{
			name:            "private task",
			text:            "/task",
			wantCommand:     "/task",
			chatType:        "private",
			wantPrompt:      "@assignee1 @assignee2",
			wantPlaceholder: "task, assignees, description",
		},
		{
			name:              "group task",
			text:              "/task@test_bot",
			wantCommand:       "/task",
			chatType:          "group",
			wantPrompt:        "@assignee1 @assignee2",
			wantPlaceholder:   "task, assignees, description",
			wantSelectiveMode: true,
		},
		{
			name:            "agenda",
			text:            "/agenda",
			wantCommand:     "/agenda",
			chatType:        "private",
			wantPrompt:      "Send the agenda",
			wantPlaceholder: "agenda",
		},
		{
			name:              "deadline keeps task reply",
			text:              "/deadline",
			wantCommand:       "/deadline",
			chatType:          "group",
			repliedToText:     "Task: https://www.notion.so/12345678123412341234123456789abc",
			wantPrompt:        "YYYY-MM-DD",
			wantPlaceholder:   "YYYY-MM-DD",
			wantReplyContext:  "Task: https://www.notion.so/12345678123412341234123456789abc",
			wantSelectiveMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &tgbotapi.Message{
				MessageID: 10,
				From:      &tgbotapi.User{ID: 20, UserName: "gibsn"},
				Chat:      &tgbotapi.Chat{ID: 30, Type: tt.chatType},
				Text:      tt.text,
				Entities:  makeBotCommandEntities(tt.text),
			}
			if tt.repliedToText != "" {
				message.ReplyToMessage = &tgbotapi.Message{
					MessageID: 9,
					Text:      tt.repliedToText,
				}
			}

			p := NewRequestProcessor(nil, "", nil)
			response, err := p.processRequest(tgbotapi.Update{Message: message})

			require.NoError(t, err)
			assert.Contains(t, response.text, tt.wantPrompt)
			assert.Contains(t, response.text, "/cancel")
			require.NotNil(t, response.forceReply)
			assert.Equal(t, tt.wantPlaceholder, response.forceReply.InputFieldPlaceholder)
			assert.Equal(t, tt.wantSelectiveMode, response.forceReply.Selective)
			require.NotNil(t, response.pending)
			assert.Equal(t, tt.wantCommand, response.pending.command)
			assert.Equal(t, tt.wantReplyContext, response.pending.repliedToText)
		})
	}
}

func TestDeadlineWithoutTaskReplyStillShowsUsageError(t *testing.T) {
	text := "/deadline"
	p := NewRequestProcessor(nil, "", nil)
	response, err := p.processRequest(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 10,
		From:      &tgbotapi.User{ID: 20, UserName: "gibsn"},
		Chat:      &tgbotapi.Chat{ID: 30, Type: "private"},
		Text:      text,
		Entities:  makeBotCommandEntities(text),
	}})

	require.Error(t, err)
	assert.Nil(t, response.forceReply)
	assert.Contains(t, response.text, "Must be a reply to a message with task link")
}

func TestPendingPrivateTaskBuildsExplicitAssigneeFormat(t *testing.T) {
	message := &tgbotapi.Message{
		From: &tgbotapi.User{ID: 20, UserName: "Gibsn"},
		Chat: &tgbotapi.Chat{ID: 30, Type: "private"},
		Text: "Task name\n@gibsn\nDescription",
	}
	pending := pendingInput{
		command:            "/task",
		repliedToText:      "original message",
		repliedToMessageID: 40,
	}

	command := pendingInputCommand(pending, message)

	assert.Equal(t, "/task", command.command)
	assert.Equal(t, "Task name\n@gibsn\nDescription", command.restOfMessage)
	assert.Equal(t, "gibsn", command.fromUserName)
	assert.Equal(t, int64(20), command.fromUserID)
	assert.Equal(t, int64(30), command.chatID)
	assert.True(t, command.isPrivate)
	assert.True(t, command.explicitAssignees)
	assert.Equal(t, "original message", command.repliedToText)
	assert.Equal(t, 40, command.repliedToMessageID)

	request, err := parseTaskCommand(command)
	require.NoError(t, err)
	assert.Equal(t, "Task name", request.TaskName)
	assert.Equal(t, []string{"@gibsn"}, request.Assignees)
	assert.Equal(t, "Description", request.Description)
}

func TestPendingCommandReplyIsHandledAsCommandInput(t *testing.T) {
	p := NewRequestProcessor(nil, "", nil)
	p.setPendingInput(30, 20, pendingInput{
		command:         "/task",
		promptMessageID: 40,
	})

	response, err := p.processMessage(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 41,
		From:      &tgbotapi.User{ID: 20, UserName: "gibsn"},
		Chat:      &tgbotapi.Chat{ID: 30, Type: "group"},
		Text:      "Task without an assignee",
		ReplyToMessage: &tgbotapi.Message{
			MessageID: 40,
		},
	}})

	require.Error(t, err)
	assert.Contains(t, response.text, "please provide the task's name and an assignee")
	assert.Contains(t, response.text, "Usage:\n/task")
}
