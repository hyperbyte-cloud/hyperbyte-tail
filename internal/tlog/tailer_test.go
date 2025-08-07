package tlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTailerReadsNewLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.log")
	if err := os.WriteFile(file, []byte("line1\n"), 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	tailer, err := NewTailer(file)
	if err != nil {
		t.Fatalf("new tailer: %v", err)
	}
	// Append a new line to trigger tail
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0644)
	defer f.Close()
	_, _ = f.WriteString("line2\n")
	// Wait briefly for goroutine to process
	time.Sleep(150 * time.Millisecond)
	lines := tailer.GetLines()
	if len(lines) == 0 {
		t.Fatalf("expected some lines, got 0")
	}
}

func TestBookmarks(t *testing.T) {
	tailer := &Tailer{Bookmarks: map[int]string{}}
	tailer.AddBookmark(3, "hello")
	b := tailer.GetBookmarks()
	if b[3] != "hello" {
		t.Fatalf("expected bookmark content 'hello', got %q", b[3])
	}
}
