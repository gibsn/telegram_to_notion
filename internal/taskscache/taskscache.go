package taskscache

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
)

type Cache struct {
	debug bool

	notion *notion.Notion
	dbID   string

	period time.Duration

	cacheLock sync.RWMutex
	cache     []notion.Task
}

func NewTasksCache(
	notion *notion.Notion, dbID string,
	period time.Duration,
) *Cache {
	c := &Cache{
		notion: notion,
		dbID:   dbID,
		period: period,
	}

	return c
}

func (c *Cache) RefreshPeriodically() {
	ticker := time.NewTicker(c.period)
	defer ticker.Stop()

	for {
		log.Printf("Will load tasks now")

		tasks, err := c.notion.LoadTasks(c.dbID)
		if err != nil {
			log.Printf("Could not load tasks: %v", err)
		}

		log.Printf("%d tasks loaded, next refresh in %s", len(tasks), c.period)

		if c.debug {
			for _, t := range tasks {
				fmt.Printf(
					"Task: %s\nAssignees: %s\nDeadline: %s\nURL: %s\n---",
					t.Title, t.Assignees, t.Deadline, t.Link)
			}
		}
		if tasks != nil {
			c.cacheLock.Lock()
			c.cache = tasks
			c.cacheLock.Unlock()
		}

		<-ticker.C
	}
}

func (c *Cache) Tasks() []notion.Task {
	var tasks []notion.Task

	c.cacheLock.RLock()
	tasks = c.cache
	c.cacheLock.RUnlock()

	return tasks
}

func (c *Cache) SetDebug(debug bool) {
	c.debug = debug
}
