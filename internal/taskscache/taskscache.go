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

func (c *Cache) RefreshCache() error {
	log.Printf("Refreshing tasks cache")

	tasks, err := c.notion.LoadTasks(c.dbID)
	if err != nil {
		return fmt.Errorf("could not load tasks: %w", err)
	}

	log.Printf("%d tasks loaded", len(tasks))

	c.cacheLock.Lock()
	c.cache = tasks
	c.cacheLock.Unlock()

	return nil
}

func (c *Cache) GetTasksForUser(userID string) ([]notion.Task, error) {
	if err := c.RefreshCache(); err != nil {
		log.Printf("Could not refresh cache: %v, using existing cache", err)
	}

	c.cacheLock.RLock()
	cachedTasks := c.cache
	c.cacheLock.RUnlock()

	// Filter tasks for the user
	userTasks := make([]notion.Task, 0)
	for _, task := range cachedTasks {
		for _, assignee := range task.Assignees {
			if assignee.ID == userID {
				userTasks = append(userTasks, task)
				break
			}
		}
	}

	return userTasks, nil
}

func (c *Cache) SetDebug(debug bool) {
	c.debug = debug
}
