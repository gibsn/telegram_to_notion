package trackscache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetTrackID(t *testing.T) {
	// Create a test cache
	cache := &Cache{
		cache: map[string]string{
			"Song One":     "id-1",
			"Another Song": "id-2",
			"Track Three":  "id-3",
			"FINAL SONG":   "id-4",
		},
	}

	tests := []struct {
		name           string
		trackName      string
		expectedID     string
		expectedExists bool
	}{
		{
			name:           "exact match - original case",
			trackName:      "Song One",
			expectedID:     "id-1",
			expectedExists: true,
		},
		{
			name:           "case insensitive match - all lowercase",
			trackName:      "song one",
			expectedID:     "id-1",
			expectedExists: true,
		},
		{
			name:           "case insensitive match - all uppercase",
			trackName:      "SONG ONE",
			expectedID:     "id-1",
			expectedExists: true,
		},
		{
			name:           "case insensitive match - mixed case",
			trackName:      "SoNg OnE",
			expectedID:     "id-1",
			expectedExists: true,
		},
		{
			name:           "exact match - another song",
			trackName:      "Another Song",
			expectedID:     "id-2",
			expectedExists: true,
		},
		{
			name:           "case insensitive match - another song lowercase",
			trackName:      "another song",
			expectedID:     "id-2",
			expectedExists: true,
		},
		{
			name:           "exact match - uppercase song",
			trackName:      "FINAL SONG",
			expectedID:     "id-4",
			expectedExists: true,
		},
		{
			name:           "case insensitive match - uppercase song lowercase",
			trackName:      "final song",
			expectedID:     "id-4",
			expectedExists: true,
		},
		{
			name:           "case insensitive match - uppercase song mixed case",
			trackName:      "Final Song",
			expectedID:     "id-4",
			expectedExists: true,
		},
		{
			name:           "non-existent track",
			trackName:      "Non Existent Song",
			expectedID:     "",
			expectedExists: false,
		},
		{
			name:           "empty track name",
			trackName:      "",
			expectedID:     "",
			expectedExists: false,
		},
		{
			name:           "partial match should not work",
			trackName:      "Song",
			expectedID:     "",
			expectedExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trackID, exists := cache.GetTrackID(tt.trackName)
			assert.Equal(t, tt.expectedID, trackID)
			assert.Equal(t, tt.expectedExists, exists)
		})
	}
}

func TestGetTrackIDWithEmptyCache(t *testing.T) {
	cache := &Cache{
		cache: map[string]string{},
	}

	trackID, exists := cache.GetTrackID("Any Song")
	assert.Equal(t, "", trackID)
	assert.Equal(t, false, exists)
}

func TestGetTrackIDWithNilCache(t *testing.T) {
	cache := &Cache{}

	trackID, exists := cache.GetTrackID("Any Song")
	assert.Equal(t, "", trackID)
	assert.Equal(t, false, exists)
}

func TestNewTracksCache(t *testing.T) {
	// Test that NewTracksCache creates a properly initialized cache
	cache := NewTracksCache(nil, "test-db-id", 5*time.Minute)

	assert.NotNil(t, cache)
	assert.Equal(t, "test-db-id", cache.dbID)
	assert.Equal(t, 5*time.Minute, cache.period)
	assert.Nil(t, cache.notion)
}
