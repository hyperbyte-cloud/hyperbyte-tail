package tlog

import (
	"bufio"
	"os"
	"regexp"
	"sync"

	"github.com/hpcloud/tail"
)

// Tailer tails a single log file and maintains recent lines and bookmarks.
type Tailer struct {
	Filename  string
	Tail      *tail.Tail
	Lines     []string
	Mutex     sync.Mutex
	Bookmarks map[int]string // line number to content
	maxLines  int
}

// NewTailer creates a Tailer and starts following the file.
func NewTailer(file string) (*Tailer, error) {
	t, err := tail.TailFile(file, tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: true,
		Poll:      true,
	})
	if err != nil {
		return nil, err
	}

	tailer := &Tailer{
		Filename:  file,
		Tail:      t,
		Lines:     []string{},
		Bookmarks: make(map[int]string),
		maxLines:  2000,
	}

	go func() {
		for line := range t.Lines {
			tailer.Mutex.Lock()
			tailer.Lines = append(tailer.Lines, line.Text)
			if len(tailer.Lines) > tailer.maxLines {
				tailer.Lines = tailer.Lines[len(tailer.Lines)-tailer.maxLines:]
			}
			tailer.Mutex.Unlock()
		}
	}()

	return tailer, nil
}

// GetLines returns a copy of the buffered lines.
func (t *Tailer) GetLines() []string {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	copyLines := make([]string, len(t.Lines))
	copy(copyLines, t.Lines)
	return copyLines
}

// AddBookmark records a bookmark for a given line.
func (t *Tailer) AddBookmark(lineNo int, content string) {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	t.Bookmarks[lineNo] = content
}

// GetBookmarks returns a copy of the bookmarks map.
func (t *Tailer) GetBookmarks() map[int]string {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	bm := make(map[int]string)
	for k, v := range t.Bookmarks {
		bm[k] = v
	}
	return bm
}

// ExportLines returns lines filtered by regex if provided.
func (t *Tailer) ExportLines(filter *regexp.Regexp) []string {
	lines := t.GetLines()
	if filter == nil {
		return lines
	}
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		if filter.MatchString(l) {
			filtered = append(filtered, l)
		}
	}
	return filtered
}

// SaveBookmarks writes bookmarks to a simple text file.
func (t *Tailer) SaveBookmarks(path string) error {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for ln, content := range t.Bookmarks {
		// lineNo\tcontent
		if _, err := w.WriteString(regexp.QuoteMeta(content) + "\n"); err != nil { // store content only
			return err
		}
		_ = ln // keep for potential future format
	}
	return w.Flush()
}
