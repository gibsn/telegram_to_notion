package main

import (
	"flag"
	"log"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/pinger"
	"github.com/gibsn/telegram_to_notion/internal/requestprocessor"
	"github.com/gibsn/telegram_to_notion/internal/taskscache"
	"github.com/gibsn/telegram_to_notion/internal/trackscache"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	var (
		debug                             bool
		botToken                          string
		notionToken                       string
		tasksDBID, tweaksDBID, tracksDBID string
		pingThreshold                     time.Duration
		pingStartingTime, pingEndTime     string
		pingPeriod                        time.Duration
		pingChatID                        int64
		pingText                          string
		tasksCachePeriod                  time.Duration
		tracksCachePeriod                 time.Duration
	)

	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.StringVar(&botToken, "telegram_token", "", "Telegram Bot Token")
	flag.StringVar(&notionToken, "notion_token", "", "Notion Integration Token")
	flag.StringVar(&tasksDBID, "tasks_db", "", "Notion Tasks Database ID")
	flag.StringVar(&tweaksDBID, "tweaks_db", "", "Notion Tweaks Database ID")
	flag.StringVar(&tracksDBID, "tracks_db", "", "Notion Tracks Database ID")
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
	flag.DurationVar(
		&tracksCachePeriod, "tracks_cache_period", 1*time.Minute, "Tracks cache refresh period",
	)
	flag.Parse()

	if botToken == "" || notionToken == "" || tasksDBID == "" || tweaksDBID == "" || tracksDBID == "" {
		log.Fatal("Required: telegram_token, notion_token, tasks_db, tweaks_db, tracks_db")
	}

	log.Printf("Will connect to Telgram")

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Could not connect to the Telegram API: %v", err)
	}

	log.Printf("Successfully connected to Telegram")

	notion := notion.NewNotion(notionToken)
	cache := taskscache.NewTasksCache(notion, tasksDBID, tasksCachePeriod)
	tracksCache := trackscache.NewTracksCache(notion, tracksDBID, tracksCachePeriod)

	processor := requestprocessor.NewRequestProcessor(notion, tasksDBID, bot)
	processor.SetTasksCache(cache)
	processor.SetTracksCache(tracksCache)
	processor.SetTweaksConfig(tweaksDBID, tracksDBID)

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
		notion.SetDebug(debug)
		processor.SetDebug(debug)
		cache.SetDebug(debug)
		tracksCache.SetDebug(debug)
		pinger.SetDebug(debug)
	}

	go processor.ProcessRequests()
	go cache.RefreshPeriodically() // TODO should start pinger only after tasks have been loaded
	go tracksCache.RefreshPeriodically()
	go pinger.PingPeriodically()

	for {
		time.Sleep(time.Second)
	}
}
