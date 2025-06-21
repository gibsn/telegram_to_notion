package main

import (
	"flag"
	"log"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/pinger"
	"github.com/gibsn/telegram_to_notion/internal/requestprocessor"
	"github.com/gibsn/telegram_to_notion/internal/taskscache"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	var (
		debug                         bool
		botToken                      string
		notionToken, notionDBID       string
		pingThreshold                 time.Duration
		pingStartingTime, pingEndTime string
		pingPeriod                    time.Duration
		pingChatID                    int64
		pingText                      string
		tasksCachePeriod              time.Duration
	)

	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.StringVar(&botToken, "telegram_token", "", "Telegram Bot Token")
	flag.StringVar(&notionToken, "notion_token", "", "Notion Integration Token")
	flag.StringVar(&notionDBID, "notion_db", "", "Notion Database ID")
	flag.DurationVar(
		&pingThreshold, "ping_threshold", 72*time.Hour, "Days till deadline when to start pinging",
	)
	flag.StringVar(&pingStartingTime, "ping_st_time", "09:00", "Time to start pinging")
	flag.StringVar(&pingEndTime, "ping_end_time", "23:00", "Time to finish pinging")
	flag.DurationVar(&pingPeriod, "ping_period_time", 6*time.Hour, "Pinging period")
	flag.Int64Var(&pingChatID, "ping_chat_id", 0, "Pinger chat ID")
	flag.StringVar(&pingText, "ping_text", "Hi, what's the estimate?", "Text for ping message")
	flag.DurationVar(
		&tasksCachePeriod, "tasks_cache_period", 1*time.Minute, "Tasks cache refresh period",
	)
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
	cache := taskscache.NewTasksCache(notion, notionDBID, tasksCachePeriod)

	processor := requestprocessor.NewRequestProcessor(notion, notionDBID, bot)
	processor.SetTasksCache(cache)

	pinger, err := pinger.NewPinger(cache, bot, pingChatID)
	if err != nil {
		log.Fatalf("Could not initialise pinger: %v", err)
	}
	if err := pinger.SetStartingTime(pingStartingTime); err != nil {
		log.Fatalf("Could not set up starting time for pinger: %v", err)
	}
	if err := pinger.SetEndTime(pingEndTime); err != nil {
		log.Fatalf("Could not set up finish time for pinger: %v", err)
	}

	pinger.SetThreshold(pingThreshold)
	pinger.SetPeriod(pingPeriod)
	pinger.SetPingText(pingText)

	if debug {
		processor.SetDebug(debug)
		cache.SetDebug(debug)
		pinger.SetDebug(debug)
	}

	go processor.ProcessRequests()
	go cache.RefreshPeriodically() // TODO should start pinger only after tasks have been loaded
	go pinger.PingPeriodically()

	for {
		time.Sleep(time.Second)
	}
}
