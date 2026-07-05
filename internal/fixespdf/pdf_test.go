package fixespdf

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	doc, err := Build("Track/Name", 2, []Row{
		{
			Summary:     "Поправить вокал",
			Start:       "0:10",
			End:         "0:20",
			Explanation: "Слишком громко\nНужен перенос",
			Author:      "Kirill",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "Правки Track-Name 2.pdf", doc.FileName)
	assert.True(t, bytes.HasPrefix(doc.Bytes, []byte("%PDF-")))
	assert.Greater(t, len(doc.Bytes), 1000)
}

func TestBuildWithLongWrappedRows(t *testing.T) {
	longText := strings.Repeat(
		"Очень длинное пояснение, которое должно переноситься внутри ячейки. ",
		80,
	)

	doc, err := Build("Long Track", 1, []Row{
		{
			Summary:     "Длинная правка без обрезания",
			Start:       "0:01",
			End:         "0:10",
			Explanation: longText,
			Author:      "Kirill",
		},
	})

	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(doc.Bytes, []byte("%PDF-")))
	assert.Greater(t, len(doc.Bytes), 1000)
}

func TestSortedRowsByStart(t *testing.T) {
	rows := []Row{
		{Summary: "third", Start: "1:05"},
		{Summary: "first", Start: "0:09"},
		{Summary: "second", Start: "0:45"},
		{Summary: "without time"},
	}

	got := sortedRowsByStart(rows)

	assert.Equal(t, []string{"first", "second", "third", "without time"}, []string{
		got[0].Summary,
		got[1].Summary,
		got[2].Summary,
		got[3].Summary,
	})
	assert.Equal(t, "third", rows[0].Summary)
}

func TestSplitCellLines(t *testing.T) {
	cells := [][]string{
		{"one", "two", "three"},
		{"single"},
		{},
	}

	chunk, rest := splitCellLines(cells, 2)

	assert.Equal(t, [][]string{{"one", "two"}, {"single"}, {}}, chunk)
	assert.Equal(t, [][]string{{"three"}, {}, {}}, rest)
}

func TestParseTimeToSeconds(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		want   int
		wantOK bool
	}{
		{name: "minutes and seconds", value: "1:05", want: 65, wantOK: true},
		{name: "zero minutes", value: "0:45", want: 45, wantOK: true},
		{name: "invalid", value: "intro", wantOK: false},
		{name: "invalid seconds", value: "1:99", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTimeToSeconds(tt.value)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
