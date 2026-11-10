package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"fqstory/internal/crawler"
	"fqstory/internal/masquerade"
	"fqstory/internal/storage"
)

func TestUIModelLifecycle(t *testing.T) {
	store, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer store.Close()

	client := crawler.NewClient()
	model := NewModel(store, client)

	// 1. Initial State
	if model.state != StateShelf {
		t.Fatalf("expected initial state to be StateShelf, got %v", model.state)
	}

	// 2. Window Resize
	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)
	if model.width != 100 || model.height != 40 {
		t.Fatalf("expected width 100, height 40, got %d, %d", model.width, model.height)
	}

	// 3. Switch to AddBook
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = m.(Model)
	if model.state != StateAddBook {
		t.Fatalf("expected StateAddBook after pressing 'a', got %v", model.state)
	}

	// 4. Cancel AddBook back to Shelf
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = m.(Model)
	if model.state != StateShelf {
		t.Fatalf("expected StateShelf after pressing Esc in AddBook, got %v", model.state)
	}

	// 5. Boss Key simulation from Reader
	model.state = StateReader
	model.currChapter = &crawler.ChapterContent{
		Title:      "测试第1章",
		Paragraphs: []string{"第一段话"},
	}
	model.currentBook = &storage.Book{
		Title: "测试书名",
	}

	// Press 'b' for Boss Key
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	model = m.(Model)
	if model.state != StateBoss {
		t.Fatalf("expected StateBoss after pressing 'b', got %v", model.state)
	}

	// In Boss screen, View should look like go test
	bossView := model.View()
	if len(bossView) == 0 {
		t.Fatalf("expected non-empty boss view")
	}

	// Press any key to restore
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = m.(Model)
	if model.state != StateReader {
		t.Fatalf("expected return to StateReader after unbossing, got %v", model.state)
	}

	// 6. Stealth Mode Cycle
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	model = m.(Model)
	if model.stealthMode != masquerade.ModeLog {
		t.Fatalf("expected ModeLog after pressing 'm', got %v", model.stealthMode)
	}

	// 7. Test TextInput typing & pasting in StateAddBook
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}) // back to shelf
	model = m.(Model)
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}) // open add book
	model = m.(Model)
	if model.state != StateAddBook {
		t.Fatalf("expected StateAddBook, got %v", model.state)
	}

	// Type characters
	for _, ch := range "6883748331202284558" {
		m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = m.(Model)
	}
	if model.textInput.Value() != "6883748331202284558" {
		t.Fatalf("expected input to contain typed ID, got: %q", model.textInput.Value())
	}

	// 8. Test BookDetail & Toc Search
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc}) // back to shelf
	model = m.(Model)
	model.books = []*storage.Book{
		{BookID: "111", Title: "测试书", ChapterCount: 10, LastReadIndex: 1},
	}
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}) // open detail
	model = m.(Model)
	if model.state != StateBookDetail {
		t.Fatalf("expected StateBookDetail after 'i', got %v", model.state)
	}

	// 9. Test TOC search filter
	model.state = StateToc
	model.chapters = []crawler.Chapter{
		{Index: 1, Title: "第一章 风起"},
		{Index: 2, Title: "第二章 云涌"},
		{Index: 3, Title: "第三章 剑仙降世"},
	}
	model.filteredChapters = model.chapters

	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // start searching
	model = m.(Model)
	if !model.tocSearching {
		t.Fatalf("expected tocSearching to be true")
	}

	// Type search query "剑仙"
	for _, ch := range "剑仙" {
		m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = m.(Model)
	}
	if len(model.filteredChapters) != 1 || model.filteredChapters[0].Index != 3 {
		t.Fatalf("expected 1 filtered chapter with Index 3, got: %+v", model.filteredChapters)
	}

	// 10. Test Single-line Ticker Offset
	model.state = StateReader
	model.stealthMode = masquerade.ModeStatusLine
	model.miniTickerOffset = 0
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = m.(Model)
	if model.miniTickerOffset != 25 {
		t.Fatalf("expected miniTickerOffset=25, got %d", model.miniTickerOffset)
	}

	// 11. Test Rankings screen
	model.state = StateShelf
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = m.(Model)
	if model.state != StateRank {
		t.Fatalf("expected StateRank after 'r', got %v", model.state)
	}

	// Mock rankLoadedMsg
	m, _ = model.Update(rankLoadedMsg{
		categoryIndex: 0,
		books: []crawler.RankBook{
			{Rank: 1, BookID: "999", Title: "榜首小说", Author: "榜首作者", ReadCount: "30万在读"},
		},
	})
	model = m.(Model)
	if len(model.rankBooks) != 1 || model.rankBooks[0].Title != "榜首小说" {
		t.Fatalf("unexpected rank books: %+v", model.rankBooks)
	}

	// Press Tab to switch category
	m, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = m.(Model)
	if model.rankCategoryIdx != 1 {
		t.Fatalf("expected categoryIndex=1 after Tab, got %d", model.rankCategoryIdx)
	}

	// View rendering
	rankView := model.View()
	if !strings.Contains(rankView, "热门排行榜") {
		t.Fatalf("expected rank view header to contain '热门排行榜'")
	}
}
