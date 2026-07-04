package fixespdf

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/signintech/gopdf"
)

const (
	pageWidth  = 956.8182
	pageHeight = 676.1364

	leftMargin   = 80.0
	rightMargin  = 80.0
	topMargin    = 88.0
	bottomMargin = 44.0

	bodyFontSize   = 10.2
	headerFontSize = 9.8
	titleFontSize  = 13.0
	bodyLeading    = 12.0
)

var filenameUnsafeRe = regexp.MustCompile(`[\\/:*?"<>|]+`)

type Row struct {
	Summary     string
	TrackPart   string
	Start       string
	End         string
	Explanation string
	Author      string
}

type Document struct {
	FileName string
	Bytes    []byte
}

func Build(track string, iteration int, rows []Row) (*Document, error) {
	if iteration <= 0 {
		return nil, fmt.Errorf("iteration must be positive")
	}
	if strings.TrimSpace(track) == "" {
		return nil, fmt.Errorf("track is required")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("rows are required")
	}

	fonts, err := resolveFonts()
	if err != nil {
		return nil, err
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: pageWidth, H: pageHeight}})
	pdf.SetMargins(leftMargin, topMargin, rightMargin, bottomMargin)
	if err := pdf.AddTTFFont("regular", fonts.regular); err != nil {
		return nil, fmt.Errorf("could not register regular font: %w", err)
	}
	if err := pdf.AddTTFFont("bold", fonts.bold); err != nil {
		return nil, fmt.Errorf("could not register bold font: %w", err)
	}

	renderer := pdfRenderer{
		pdf:   pdf,
		title: "Правки " + strconv.Itoa(iteration),
	}
	if err := renderer.addPage(); err != nil {
		return nil, err
	}
	if err := renderer.drawTable(sortedRowsByStart(rows)); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("could not write pdf: %w", err)
	}

	return &Document{
		FileName: fmt.Sprintf("Правки %s %d.pdf", sanitizeTrackName(track), iteration),
		Bytes:    buf.Bytes(),
	}, nil
}

type fontPair struct {
	regular string
	bold    string
}

func resolveFonts() (fontPair, error) {
	if fromEnv, ok := fontPairFromEnv(); ok {
		return fromEnv, nil
	}

	candidates := []fontPair{
		{
			regular: "/System/Library/Fonts/Supplemental/Arial.ttf",
			bold:    "/System/Library/Fonts/Supplemental/Arial Bold.ttf",
		},
		{
			regular: "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			bold:    "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
		},
	}

	for _, candidate := range candidates {
		if fileExists(candidate.regular) && fileExists(candidate.bold) {
			return candidate, nil
		}
	}

	return fontPair{}, fmt.Errorf("could not find a TTF font with Cyrillic support")
}

func fontPairFromEnv() (fontPair, bool) {
	regular := os.Getenv("FIXESPDF_REGULAR_FONT")
	bold := os.Getenv("FIXESPDF_BOLD_FONT")
	if regular == "" || bold == "" {
		return fontPair{}, false
	}
	if !fileExists(regular) || !fileExists(bold) {
		return fontPair{}, false
	}
	return fontPair{regular: regular, bold: bold}, true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func sanitizeTrackName(track string) string {
	cleaned := filenameUnsafeRe.ReplaceAllString(strings.TrimSpace(track), "-")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	cleaned = strings.Trim(cleaned, " .-")
	if cleaned == "" {
		return "track"
	}
	return cleaned
}

type pdfRenderer struct {
	pdf    *gopdf.GoPdf
	title  string
	pageNo int
	y      float64
}

type column struct {
	title string
	width float64
}

var tableColumns = []column{
	{title: "Кратко", width: 310},
	{title: "Начало", width: 42},
	{title: "Конец", width: 44},
	{title: "Пояснение", width: 291},
	{title: "Автор", width: 104},
}

func sortedRowsByStart(rows []Row) []Row {
	sorted := append([]Row(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, leftOK := parseTimeToSeconds(sorted[i].Start)
		right, rightOK := parseTimeToSeconds(sorted[j].Start)

		switch {
		case leftOK && rightOK:
			return left < right
		case leftOK:
			return true
		case rightOK:
			return false
		default:
			return strings.TrimSpace(sorted[i].Start) < strings.TrimSpace(sorted[j].Start)
		}
	})
	return sorted
}

func parseTimeToSeconds(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}

	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.Atoi(parts[1])
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, false
	}

	return minutes*60 + seconds, true
}

func (r *pdfRenderer) addPage() error {
	r.pageNo++
	r.pdf.AddPage()
	if err := r.drawFooter(); err != nil {
		return err
	}
	r.y = topMargin
	return r.drawTitle()
}

func (r *pdfRenderer) drawFooter() error {
	if err := r.pdf.SetFont("regular", "", titleFontSize); err != nil {
		return err
	}
	r.pdf.SetXY(0, 24)
	return r.pdf.CellWithOption(
		&gopdf.Rect{W: pageWidth, H: 16},
		strconv.Itoa(r.pageNo),
		gopdf.CellOption{Align: gopdf.Center},
	)
}

func (r *pdfRenderer) drawTitle() error {
	if err := r.pdf.SetFont("regular", "", titleFontSize); err != nil {
		return err
	}
	r.pdf.SetXY(0, r.y)
	if err := r.pdf.CellWithOption(
		&gopdf.Rect{W: pageWidth, H: 15},
		r.title,
		gopdf.CellOption{Align: gopdf.Center},
	); err != nil {
		return err
	}
	r.y += 26
	return nil
}

func (r *pdfRenderer) drawTable(rows []Row) error {
	if err := r.drawHeader(); err != nil {
		return err
	}

	for _, row := range rows {
		cells := []string{row.Summary, row.Start, row.End, row.Explanation, row.Author}
		lines, err := r.cellLines(cells, false)
		if err != nil {
			return err
		}
		for hasAnyLines(lines) {
			if r.maxLinesOnCurrentPage() == 0 {
				if err := r.addPage(); err != nil {
					return err
				}
				if err := r.drawHeader(); err != nil {
					return err
				}
			}

			chunk, rest := splitCellLines(lines, r.maxLinesOnCurrentPage())
			height := rowHeightByLineCount(maxCellLines(chunk), false)
			if err := r.drawRowLines(chunk, height, false); err != nil {
				return err
			}
			lines = rest
		}
	}

	return nil
}

func (r *pdfRenderer) drawHeader() error {
	titles := make([]string, 0, len(tableColumns))
	for _, col := range tableColumns {
		titles = append(titles, col.title)
	}
	lines, err := r.cellLines(titles, true)
	if err != nil {
		return err
	}
	height := rowHeightByLineCount(maxCellLines(lines), true)
	if err := r.drawRowLines(lines, height, true); err != nil {
		return err
	}
	r.pdf.SetStrokeColor(111, 111, 111)
	r.pdf.SetLineWidth(0.85)
	r.pdf.Line(leftMargin, r.y, leftMargin+tableWidth(), r.y)
	return nil
}

func (r *pdfRenderer) cellLines(cells []string, header bool) ([][]string, error) {
	fontSize := bodyFontSize
	if header {
		fontSize = headerFontSize
	}
	lines := make([][]string, 0, len(cells))
	for i, text := range cells {
		if err := r.pdf.SetFont(fontForCell(i, header), "", fontSize); err != nil {
			return nil, err
		}
		cellLines, err := splitCellText(r.pdf, text, tableColumns[i].width-8)
		if err != nil {
			return nil, err
		}
		lines = append(lines, cellLines)
	}

	return lines, nil
}

func rowHeightByLineCount(lineCount int, header bool) float64 {
	height := float64(lineCount)*bodyLeading + 10
	if header {
		height += 1
	}
	return height
}

func (r *pdfRenderer) drawRowLines(cells [][]string, height float64, header bool) error {
	x := leftMargin
	for i, lines := range cells {
		switch {
		case header:
			r.pdf.SetFillColor(191, 193, 193)
		case i == 0:
			r.pdf.SetFillColor(221, 221, 221)
		default:
			r.pdf.SetFillColor(255, 255, 255)
		}

		r.pdf.SetStrokeColor(200, 200, 200)
		r.pdf.SetLineWidth(0.45)
		r.pdf.RectFromUpperLeftWithStyle(x, r.y, tableColumns[i].width, height, "DF")

		fontSize := bodyFontSize
		if header {
			fontSize = headerFontSize
		}
		if err := r.pdf.SetFont(fontForCell(i, header), "", fontSize); err != nil {
			return err
		}
		r.pdf.SetTextColor(0, 0, 0)
		for lineIndex, line := range lines {
			r.pdf.SetXY(x+4, r.y+5+bodyFontSize+float64(lineIndex)*bodyLeading)
			if err := r.pdf.Text(line); err != nil {
				return err
			}
		}

		x += tableColumns[i].width
	}
	r.y += height
	return nil
}

func (r *pdfRenderer) maxLinesOnCurrentPage() int {
	available := pageHeight - bottomMargin - r.y - 10
	if available < bodyLeading {
		return 0
	}
	return int(available / bodyLeading)
}

func splitCellText(pdf *gopdf.GoPdf, text string, width float64) ([]string, error) {
	parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			lines = append(lines, "")
			continue
		}
		split, err := pdf.SplitTextWithWordWrap(part, width)
		if err != nil {
			return nil, err
		}
		lines = append(lines, split...)
	}
	if len(lines) == 0 {
		return []string{""}, nil
	}
	return lines, nil
}

func hasAnyLines(cells [][]string) bool {
	for _, lines := range cells {
		if len(lines) > 0 {
			return true
		}
	}
	return false
}

func splitCellLines(cells [][]string, maxLines int) ([][]string, [][]string) {
	chunk := make([][]string, 0, len(cells))
	rest := make([][]string, 0, len(cells))
	for _, lines := range cells {
		splitAt := maxLines
		if len(lines) < splitAt {
			splitAt = len(lines)
		}
		chunk = append(chunk, lines[:splitAt])
		rest = append(rest, lines[splitAt:])
	}
	return chunk, rest
}

func maxCellLines(cells [][]string) int {
	maxLines := 1
	for _, lines := range cells {
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	return maxLines
}

func fontForCell(index int, header bool) string {
	if header || index == 0 {
		return "bold"
	}
	return "regular"
}

func tableWidth() float64 {
	width := 0.0
	for _, col := range tableColumns {
		width += col.width
	}
	return width
}
