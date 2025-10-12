package trackscache

import (
	"fmt"
	"log"
	"strings"
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
	cache     map[string]string // track title -> track ID
}

func NewTracksCache(
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
		log.Printf("Will load tracks now")

		tracks, err := c.notion.LoadTracks(c.dbID)
		if err != nil {
			log.Printf("Could not load tracks: %v", err)
		}

		log.Printf("%d tracks loaded, next refresh in %s", len(tracks), c.period)

		if tracks != nil {
			c.cacheLock.Lock()
			c.cache = tracks
			c.cacheLock.Unlock()
		}

		<-ticker.C
	}
}

func (c *Cache) GetTracks() map[string]string {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()
	return c.cache
}

func (c *Cache) GetTrackID(trackName string) (string, bool) {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	// First try exact match
	trackID, exists := c.cache[trackName]
	if exists {
		return trackID, true
	}

	// if exact match fails, try case-insensitive search
	lowerTrackName := strings.ToLower(trackName)
	for name, id := range c.cache {
		if strings.ToLower(name) == lowerTrackName {
			return id, true
		}
	}

	return "", false
}

func (c *Cache) GetTrackNames() []string {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	names := make([]string, 0, len(c.cache))
	for name := range c.cache {
		names = append(names, name)
	}
	return names
}

func (c *Cache) RefreshCache() error {
	log.Printf("Refreshing tracks cache")

	tracks, err := c.notion.LoadTracks(c.dbID)
	if err != nil {
		return fmt.Errorf("could not load tracks: %w", err)
	}

	log.Printf("%d tracks loaded", len(tracks))

	c.cacheLock.Lock()
	c.cache = tracks
	c.cacheLock.Unlock()

	return nil
}

func (c *Cache) SetDebug(debug bool) {
	c.debug = debug
}
