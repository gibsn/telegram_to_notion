package requestprocessor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type telegramRequest struct {
	method string
	form   url.Values
}

type fakeTracksCache struct {
	tracks map[string]string
}

func (f *fakeTracksCache) GetTrackID(name string) (string, bool) {
	id, ok := f.tracks[name]
	return id, ok
}

func (f *fakeTracksCache) GetTrackName(id string) (string, bool) {
	for name, trackID := range f.tracks {
		if trackID == id {
			return name, true
		}
	}
	return "", false
}

func (f *fakeTracksCache) GetTrackNames() []string {
	return []string{"Track One"}
}

func TestProcessTweakCallbackPromptsAndStoresConversation(t *testing.T) {
	var requests []telegramRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse Telegram request form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests = append(requests, telegramRequest{method: r.URL.Path, form: r.Form})
		w.Header().Set("Content-Type", "application/json")

		result := interface{}(true)
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			result = map[string]interface{}{
				"id": 1, "is_bot": true, "first_name": "test", "username": "test_bot",
			}
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			result = map[string]interface{}{
				"message_id": 99,
				"date":       1,
				"chat":       map[string]interface{}{"id": 30, "type": "private"},
			}
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true, "result": result,
		}); err != nil {
			t.Errorf("encode Telegram response: %v", err)
		}
	}))
	defer server.Close()

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("test-token", server.URL+"/bot%s/%s")
	require.NoError(t, err)
	p := NewRequestProcessor(nil, "", bot)
	p.tracksCache = &fakeTracksCache{tracks: map[string]string{"Track One": "track-id"}}
	p.processCallbackQuery(&tgbotapi.CallbackQuery{
		ID:   "callback-id",
		From: &tgbotapi.User{ID: 20, UserName: "gibsn"},
		Data: "tweak:render",
		Message: &tgbotapi.Message{
			MessageID: 10,
			Chat:      &tgbotapi.Chat{ID: 30, Type: "private"},
		},
	})

	require.Len(t, requests, 3)
	assert.True(t, strings.HasSuffix(requests[1].method, "/answerCallbackQuery"))
	assert.Equal(t, "callback-id", requests[1].form.Get("callback_query_id"))
	assert.True(t, strings.HasSuffix(requests[2].method, "/sendMessage"))
	assert.Contains(t, requests[2].form.Get("text"), "Choose a track")
	assert.Contains(t, requests[2].form.Get("reply_markup"), "Track One")

	p.processCallbackQuery(&tgbotapi.CallbackQuery{
		ID:   "track-callback-id",
		From: &tgbotapi.User{ID: 20, UserName: "gibsn"},
		Data: "twtrk:render:track-id",
		Message: &tgbotapi.Message{
			MessageID: 99,
			Chat:      &tgbotapi.Chat{ID: 30, Type: "private"},
		},
	})

	require.Len(t, requests, 5)
	assert.True(t, strings.HasSuffix(requests[3].method, "/answerCallbackQuery"))
	assert.True(t, strings.HasSuffix(requests[4].method, "/sendMessage"))
	assert.Contains(t, requests[4].form.Get("text"), "iteration number")
	assert.Contains(t, requests[4].form.Get("reply_markup"), `"force_reply":true`)

	reply := &tgbotapi.Message{
		From:           &tgbotapi.User{ID: 20},
		Chat:           &tgbotapi.Chat{ID: 30},
		ReplyToMessage: &tgbotapi.Message{MessageID: 99},
	}
	pending, found, expired := p.takePendingInput(reply)
	assert.True(t, found)
	assert.False(t, expired)
	assert.Equal(t, tweakActionRender, pending.action)
	assert.Equal(t, "Track One", pending.trackName)
}

func TestProcessTweakWithoutArgumentsShowsMenu(t *testing.T) {
	p := NewRequestProcessor(nil, "", nil)
	text := "/tweak"
	update := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 10,
		From:      &tgbotapi.User{ID: 20, UserName: "gibsn"},
		Chat:      &tgbotapi.Chat{ID: 30, Type: "private"},
		Text:      text,
		Entities:  makeBotCommandEntities(text),
	}}

	response, err := p.processRequest(update)

	require.NoError(t, err)
	assert.Equal(t, "Choose an action for /tweak:", response.text)
	require.NotNil(t, response.replyMarkup)
	require.Len(t, response.replyMarkup.InlineKeyboard, 2)

	var callbackData []string
	for _, row := range response.replyMarkup.InlineKeyboard {
		for _, button := range row {
			require.NotNil(t, button.CallbackData)
			callbackData = append(callbackData, *button.CallbackData)
		}
	}
	assert.Equal(t, []string{"tweak:demo", "tweak:mix", "tweak:render", "tweak:towork"}, callbackData)
}

func TestIsTweakMenuCommand(t *testing.T) {
	assert.True(t, isTweakMenuCommand(commandCommon{}))
	assert.True(t, isTweakMenuCommand(commandCommon{restOfMessage: "  \n\t"}))
	assert.False(t, isTweakMenuCommand(commandCommon{restOfMessage: "render Track 1"}))
}

func TestHasPendingTweakReply(t *testing.T) {
	p := NewRequestProcessor(nil, "", nil)
	message := &tgbotapi.Message{
		From:           &tgbotapi.User{ID: 20, UserName: "gibsn"},
		Chat:           &tgbotapi.Chat{ID: 30, Type: "private"},
		Text:           "ordinary message",
		ReplyToMessage: &tgbotapi.Message{MessageID: 40},
	}

	assert.False(t, p.hasPendingInputReply(message))

	p.setPendingInput(30, 20, pendingInput{action: tweakActionRender, promptMessageID: 40})
	assert.True(t, p.hasPendingInputReply(message))

	message.ReplyToMessage.MessageID = 41
	assert.False(t, p.hasPendingInputReply(message))
}

func TestManualTweakCommandStillUsesExistingParser(t *testing.T) {
	p := NewRequestProcessor(nil, "", nil)
	text := "/tweak render track invalid"
	update := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 10,
		From:      &tgbotapi.User{ID: 20, UserName: "gibsn"},
		Chat:      &tgbotapi.Chat{ID: 30, Type: "private"},
		Text:      text,
		Entities:  makeBotCommandEntities(text),
	}}

	response, err := p.processRequest(update)

	require.Error(t, err)
	assert.Nil(t, response.replyMarkup)
	assert.Contains(t, response.text, "invalid iteration number")
	assert.Contains(t, response.text, "/tweak render")
}

func TestPendingTweakCommandBuildsExistingCommandFormat(t *testing.T) {
	message := &tgbotapi.Message{
		From: &tgbotapi.User{ID: 20, UserName: "Gibsn"},
		Chat: &tgbotapi.Chat{ID: 30, Type: "group"},
		Text: "edit name\n0:05 0:10\ndescription",
	}
	pending := pendingInput{
		action:             tweakActionMix,
		trackName:          "track name",
		repliedToText:      "original message",
		repliedToMessageID: 40,
	}

	command := pendingInputCommand(pending, message)

	assert.Equal(t, "/tweak", command.command)
	assert.Equal(t, "mix track name\nedit name\n0:05 0:10\ndescription", command.restOfMessage)
	assert.Equal(t, "gibsn", command.fromUserName)
	assert.Equal(t, int64(20), command.fromUserID)
	assert.Equal(t, int64(30), command.chatID)
	assert.False(t, command.isPrivate)
	assert.Equal(t, "original message", command.repliedToText)
	assert.Equal(t, 40, command.repliedToMessageID)
}

func TestPendingTweakLifecycle(t *testing.T) {
	p := NewRequestProcessor(nil, "", nil)
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	p.setPendingInput(30, 20, pendingInput{action: tweakActionRender, promptMessageID: 40})

	wrongReply := &tgbotapi.Message{
		From:           &tgbotapi.User{ID: 20},
		Chat:           &tgbotapi.Chat{ID: 30},
		ReplyToMessage: &tgbotapi.Message{MessageID: 41},
	}
	_, found, expired := p.takePendingInput(wrongReply)
	assert.False(t, found)
	assert.False(t, expired)

	correctReply := &tgbotapi.Message{
		From:           &tgbotapi.User{ID: 20},
		Chat:           &tgbotapi.Chat{ID: 30},
		ReplyToMessage: &tgbotapi.Message{MessageID: 40},
	}
	pending, found, expired := p.takePendingInput(correctReply)
	assert.True(t, found)
	assert.False(t, expired)
	assert.Equal(t, tweakActionRender, pending.action)

	_, found, _ = p.takePendingInput(correctReply)
	assert.False(t, found)
}

func TestPendingTweakExpiresAndCanBeCancelled(t *testing.T) {
	p := NewRequestProcessor(nil, "", nil)
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	p.setPendingInput(30, 20, pendingInput{action: tweakActionDemo, promptMessageID: 40})

	assert.Equal(t, "Action cancelled.", p.processCancel(commandCommon{chatID: 30, fromUserID: 20}))
	assert.Equal(
		t,
		"There is no active action.",
		p.processCancel(commandCommon{chatID: 30, fromUserID: 20}),
	)

	p.setPendingInput(30, 20, pendingInput{action: tweakActionDemo, promptMessageID: 40})
	now = now.Add(conversationTTL)
	reply := &tgbotapi.Message{
		From:           &tgbotapi.User{ID: 20},
		Chat:           &tgbotapi.Chat{ID: 30},
		ReplyToMessage: &tgbotapi.Message{MessageID: 40},
	}

	_, found, expired := p.takePendingInput(reply)
	assert.True(t, found)
	assert.True(t, expired)
}

func TestParseTweakCallback(t *testing.T) {
	for _, action := range []tweakAction{
		tweakActionDemo,
		tweakActionMix,
		tweakActionRender,
		tweakActionToWork,
	} {
		got, ok := parseTweakCallback(tweakCallbackPrefix + string(action))
		assert.True(t, ok)
		assert.Equal(t, action, got)
	}

	_, ok := parseTweakCallback("tweak:unknown")
	assert.False(t, ok)
	_, ok = parseTweakCallback("other:render")
	assert.False(t, ok)

	action, trackID, ok := parseTweakTrackCallback("twtrk:mix:track-id")
	assert.True(t, ok)
	assert.Equal(t, tweakActionMix, action)
	assert.Equal(t, "track-id", trackID)
	_, _, ok = parseTweakTrackCallback("twtrk:unknown:track-id")
	assert.False(t, ok)
}
