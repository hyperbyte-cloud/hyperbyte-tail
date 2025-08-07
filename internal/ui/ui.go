package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"hyperbyte-logs/internal/highlight"
	"hyperbyte-logs/internal/tlog"
)

// AppUI holds the visual elements and state for the TUI.
type AppUI struct {
	App           *tview.Application
	Tailers       []*tlog.Tailer
	TextViews     []*tview.TextView
	Root          *tview.Flex
	StatusBar     *tview.TextView
	BookmarksList *tview.List

	CurrentIndex  int
	ScrollEnabled bool
	FilterRegex   *regexp.Regexp

	PageSize    int
	CurrentPage int
	NumPages    int

	PausePanel    map[int]bool // per-panel pause toggles
	OverlayActive bool
}

// NewAppUI constructs the UI with given tailers.
func NewAppUI(tailers []*tlog.Tailer) *AppUI {
	app := tview.NewApplication()
	textViews := make([]*tview.TextView, len(tailers))
	for i := range tailers {
		tv := tview.NewTextView()
		tv.SetDynamicColors(true)
		tv.SetScrollable(true)
		tv.SetWrap(false)
		tv.SetBorder(true)
		tv.SetTitle(tailers[i].Filename)
		textViews[i] = tv
	}

	status := tview.NewTextView()
	status.SetDynamicColors(true)
	status.SetBorder(true)
	status.SetTitle("Status")

	ui := &AppUI{
		App:           app,
		Tailers:       tailers,
		TextViews:     textViews,
		StatusBar:     status,
		BookmarksList: tview.NewList(),
		CurrentIndex:  0,
		ScrollEnabled: true,
		PageSize:      4,
		CurrentPage:   0,
		PausePanel:    make(map[int]bool),
	}

	ui.NumPages = (len(ui.TextViews) + ui.PageSize - 1) / ui.PageSize
	ui.BookmarksList.SetBorder(true)
	ui.BookmarksList.SetTitle("Bookmarks")
	ui.BookmarksList.SetDoneFunc(func() {
		ui.OverlayActive = false
		ui.App.SetRoot(ui.Root, true).SetFocus(ui.TextViews[ui.CurrentIndex])
	})

	ui.renderPage()
	ui.bindKeys()
	ui.loop()
	return ui
}

// Run starts the application.
func (u *AppUI) Run() error {
	return u.App.Run()
}

func (u *AppUI) renderPage() {
	topRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	bottomRow := tview.NewFlex().SetDirection(tview.FlexColumn)

	start := u.CurrentPage * u.PageSize
	end := start + u.PageSize
	if end > len(u.TextViews) {
		end = len(u.TextViews)
	}
	if start < end {
		topRow.AddItem(u.TextViews[start], 0, 1, false)
	}
	if start+1 < end {
		topRow.AddItem(u.TextViews[start+1], 0, 1, false)
	}
	if start+2 < end {
		bottomRow.AddItem(u.TextViews[start+2], 0, 1, false)
	}
	if start+3 < end {
		bottomRow.AddItem(u.TextViews[start+3], 0, 1, false)
	}

	grid := tview.NewFlex().SetDirection(tview.FlexRow)
	grid.AddItem(topRow, 0, 1, false)
	grid.AddItem(bottomRow, 0, 1, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(grid, 0, 1, false)
	root.AddItem(u.StatusBar, 1, 0, false)
	u.Root = root

	// Clamp current index within page
	if u.CurrentIndex < start || u.CurrentIndex >= end {
		u.CurrentIndex = start
	}
	u.App.SetRoot(u.Root, true).SetFocus(u.TextViews[u.CurrentIndex])
	u.renderStatus()
}

func (u *AppUI) renderStatus() {
	pageInfo := fmt.Sprintf("Page %d/%d  ", u.CurrentPage+1, max(1, u.NumPages))
	focusInfo := fmt.Sprintf("Focus: %d  ", u.CurrentIndex+1)
	pause := ""
	if u.PausePanel[u.CurrentIndex] {
		pause = "[yellow]PAUSED[-]  "
	}
	filter := ""
	if u.FilterRegex != nil {
		filter = fmt.Sprintf("Filter: /%s/  ", u.FilterRegex.String())
	}
	u.StatusBar.SetText(pageInfo + focusInfo + pause + filter + "Keys: Tab focus, [/] filter, p pause, [ ] pages, : command, h help")
}

func (u *AppUI) loop() {
	go func() {
		for {
			for i, tailer := range u.Tailers {
				if u.PausePanel[i] {
					continue
				}
				lines := tailer.GetLines()
				var sb strings.Builder
				for lineNo, line := range lines {
					if u.FilterRegex != nil && !u.FilterRegex.MatchString(line) {
						continue
					}
					sb.WriteString(highlight.ColorizeLine(line))
					sb.WriteString("\n")
					tailer.Mutex.Lock()
					_, bookmarked := tailer.Bookmarks[lineNo]
					tailer.Mutex.Unlock()
					if bookmarked {
						sb.WriteString("[green](BOOKMARK)[-]\n")
					}
				}
				idx := i
				content := sb.String()
				u.App.QueueUpdateDraw(func() {
					u.TextViews[idx].SetText(content)
					if u.ScrollEnabled && u.App.GetFocus() == u.TextViews[idx] {
						u.TextViews[idx].ScrollToEnd()
					}
				})
			}
			time.Sleep(time.Second)
		}
	}()
}

func (u *AppUI) bindKeys() {
	u.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Do not handle global shortcuts when an overlay/modal is active
		if u.OverlayActive {
			return event
		}
		tv := u.TextViews[u.CurrentIndex]
		tailer := u.Tailers[u.CurrentIndex]

		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'q':
				u.App.Stop()
			case 's':
				u.ScrollEnabled = !u.ScrollEnabled
			case 'c':
				tv.Clear()
			case 'h':
				showHelp(u)
			case '/':
				showRegexInput(u, func(r *regexp.Regexp) {
					u.FilterRegex = r
					u.renderStatus()
				})
			case 'b':
				lines := tailer.GetLines()
				ln := len(lines) - 1
				if ln >= 0 {
					tailer.AddBookmark(ln, lines[ln])
					showBookmarks(u, tailer, tv)
				}
			case 'B':
				showBookmarks(u, tailer, tv)
			case '[':
				if u.NumPages > 1 {
					u.CurrentPage--
					if u.CurrentPage < 0 {
						u.CurrentPage = u.NumPages - 1
					}
					u.CurrentIndex = u.CurrentPage * u.PageSize
					u.renderPage()
				}
			case ']':
				if u.NumPages > 1 {
					u.CurrentPage = (u.CurrentPage + 1) % u.NumPages
					u.CurrentIndex = u.CurrentPage * u.PageSize
					u.renderPage()
				}
			case 'p':
				u.PausePanel[u.CurrentIndex] = !u.PausePanel[u.CurrentIndex]
				u.renderStatus()
			case ':':
				// Command palette
				openCommandPalette(u)
			}
		} else if event.Key() == tcell.KeyTAB {
			u.switchFocus(1)
		}
		return event
	})
}

func (u *AppUI) switchFocus(dir int) {
	u.CurrentIndex += dir
	if u.CurrentIndex < 0 {
		u.CurrentIndex = len(u.TextViews) - 1
	} else if u.CurrentIndex >= len(u.TextViews) {
		u.CurrentIndex = 0
	}
	newPage := 0
	if u.PageSize > 0 {
		newPage = u.CurrentIndex / u.PageSize
	}
	if newPage != u.CurrentPage {
		u.CurrentPage = newPage
		u.renderPage()
		return
	}
	u.App.SetFocus(u.TextViews[u.CurrentIndex])
	u.renderStatus()
}

// Helpers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Dialogs and palettes

// showRegexInput opens an overlay input for regex filtering and blocks global shortcuts.
func showRegexInput(u *AppUI, setFilter func(*regexp.Regexp)) {
	input := tview.NewInputField()
	input.SetLabel("Regex filter (empty to clear): ")
	input.SetDoneFunc(func(key tcell.Key) {
		text := input.GetText()
		if text == "" {
			setFilter(nil)
		} else {
			re, err := regexp.Compile(text)
			if err != nil {
				input.SetLabel(fmt.Sprintf("Invalid regex: %v. Try again: ", err))
				u.App.Draw()
				return
			}
			setFilter(re)
		}
		u.OverlayActive = false
		u.App.SetRoot(u.Root, true).SetFocus(u.TextViews[u.CurrentIndex])
	})
	u.OverlayActive = true
	u.App.SetRoot(input, true).SetFocus(input)
}

// showBookmarks opens an overlay list of bookmarks for a tailer and blocks global shortcuts.
func showBookmarks(u *AppUI, tailer *tlog.Tailer, targetView *tview.TextView) {
	u.BookmarksList.Clear()
	bookmarks := tailer.GetBookmarks()
	for lineNo, line := range bookmarks {
		display := fmt.Sprintf("Line %d: %.60s", lineNo+1, line)
		ln := lineNo
		u.BookmarksList.AddItem(display, "", 0, func() {
			u.OverlayActive = false
			u.App.SetRoot(u.Root, true).SetFocus(targetView)
			targetView.ScrollTo(ln, 0)
		})
	}
	u.BookmarksList.AddItem("Close", "Back to logs", 'q', func() {
		u.OverlayActive = false
		u.App.SetRoot(u.Root, true).SetFocus(targetView)
	})
	u.OverlayActive = true
	u.App.SetRoot(u.BookmarksList, true).SetFocus(u.BookmarksList)
}

// openCommandPalette opens a simple palette with commands for export, clear, save bookmarks.
func openCommandPalette(u *AppUI) {
	form := tview.NewForm()
	filterStr := ""
	if u.FilterRegex != nil {
		filterStr = u.FilterRegex.String()
	}
	form.AddInputField("Filter (regex)", filterStr, 40, nil, func(text string) {
		if text == "" {
			u.FilterRegex = nil
		} else if re, err := regexp.Compile(text); err == nil {
			u.FilterRegex = re
		}
		u.renderStatus()
	})
	form.AddButton("Export Visible Lines", func() {
		lines := u.Tailers[u.CurrentIndex].ExportLines(u.FilterRegex)
		// show in a modal text view
		tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(false)
		tv.SetBorder(true).SetTitle("Export (press any key to close)")
		tv.SetText(strings.Join(lines, "\n"))
		tv.SetDoneFunc(func(key tcell.Key) {
			u.OverlayActive = false
			u.App.SetRoot(u.Root, true).SetFocus(u.TextViews[u.CurrentIndex])
		})
		u.OverlayActive = true
		u.App.SetRoot(tv, true).SetFocus(tv)
	})
	form.AddButton("Clear Current View", func() {
		u.TextViews[u.CurrentIndex].Clear()
		u.OverlayActive = false
		u.App.SetRoot(u.Root, true).SetFocus(u.TextViews[u.CurrentIndex])
	})
	form.AddButton("Save Bookmarks", func() {
		_ = u.Tailers[u.CurrentIndex].SaveBookmarks("bookmarks.txt")
		u.OverlayActive = false
		u.App.SetRoot(u.Root, true).SetFocus(u.TextViews[u.CurrentIndex])
	})
	form.AddButton("Close", func() { u.OverlayActive = false; u.App.SetRoot(u.Root, true).SetFocus(u.TextViews[u.CurrentIndex]) })
	form.SetBorder(true).SetTitle(": Command Palette")
	u.OverlayActive = true
	u.App.SetRoot(form, true).SetFocus(form)
}

func showHelp(u *AppUI) {
	help := tview.NewModal().
		SetText(`Key bindings:

q - Quit
s - Toggle auto-scroll
c - Clear current log view
/ - Regex filter (empty clears)
b - Bookmark last line in current log
B - Show bookmarks list
Tab - Switch log panel focus
h - Show help
[ - Previous page (if more than 4 logs)
] - Next page (if more than 4 logs)
p - Pause/resume current panel
: - Command palette`).
		AddButtons([]string{"Close"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			u.OverlayActive = false
			u.App.SetRoot(u.Root, true).SetFocus(u.TextViews[u.CurrentIndex])
		})
	u.OverlayActive = true
	u.App.SetRoot(help, false).SetFocus(help)
}
