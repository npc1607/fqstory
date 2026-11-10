package storage

import (
	"testing"
)

func TestStorageMemory(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory storage: %v", err)
	}
	defer s.Close()

	// 1. Save book
	b := &Book{
		BookID:        "1234567890",
		Title:         "测试小说",
		Author:        "测试作者",
		Score:         "9.5",
		Intro:         "测试简介",
		ChapterCount:  100,
		LastReadIndex: 1,
		LastReadTitle: "第1章",
	}
	if err := s.SaveBook(b); err != nil {
		t.Fatalf("SaveBook failed: %v", err)
	}

	// 2. Get book
	got, err := s.GetBook("1234567890")
	if err != nil || got == nil {
		t.Fatalf("GetBook failed: %v, got %v", err, got)
	}
	if got.Title != "测试小说" || got.LastReadIndex != 1 {
		t.Fatalf("unexpected book content: %+v", got)
	}

	// 3. Update Progress
	if err := s.UpdateProgress("1234567890", 5, "第5章 大显身手", 42); err != nil {
		t.Fatalf("UpdateProgress failed: %v", err)
	}
	got2, _ := s.GetBook("1234567890")
	if got2.LastReadIndex != 5 || got2.LastReadTitle != "第5章 大显身手" || got2.LastReadOffset != 42 {
		t.Fatalf("progress update mismatch: %+v", got2)
	}

	// 4. Cache Chapter
	paras := []string{"第一段话。", "第二段话。"}
	if err := s.SaveChapter("1234567890", 5, "item_555", "第5章 大显身手", paras, 20); err != nil {
		t.Fatalf("SaveChapter failed: %v", err)
	}

	if !s.HasChapter("1234567890", 5) {
		t.Fatalf("expected HasChapter to be true")
	}

	ch, err := s.GetChapter("1234567890", 5)
	if err != nil || ch == nil {
		t.Fatalf("GetChapter failed: %v", err)
	}
	if len(ch.Paragraphs) != 2 || ch.Paragraphs[0] != "第一段话。" {
		t.Fatalf("unexpected cached paragraphs: %+v", ch)
	}

	// 5. Delete Book
	if err := s.DeleteBook("1234567890"); err != nil {
		t.Fatalf("DeleteBook failed: %v", err)
	}
	if s.HasChapter("1234567890", 5) {
		t.Fatalf("expected chapters to be deleted")
	}
	got3, _ := s.GetBook("1234567890")
	if got3 != nil {
		t.Fatalf("expected book to be deleted")
	}
}
