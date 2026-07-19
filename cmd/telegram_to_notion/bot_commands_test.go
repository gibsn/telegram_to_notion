package main

import (
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTelegramRequester struct {
	request tgbotapi.Chattable
	err     error
}

func (f *fakeTelegramRequester) Request(request tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	f.request = request
	return &tgbotapi.APIResponse{Ok: f.err == nil}, f.err
}

func TestRegisterBotCommands(t *testing.T) {
	bot := &fakeTelegramRequester{}

	err := registerBotCommands(bot)
	require.NoError(t, err)

	config, ok := bot.request.(tgbotapi.SetMyCommandsConfig)
	require.True(t, ok)
	assert.Equal(t, botCommands(), config.Commands)
}

func TestRegisterBotCommandsReturnsTelegramError(t *testing.T) {
	telegramErr := errors.New("telegram unavailable")
	bot := &fakeTelegramRequester{err: telegramErr}

	err := registerBotCommands(bot)

	require.Error(t, err)
	assert.ErrorIs(t, err, telegramErr)
}
