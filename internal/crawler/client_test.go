package crawler

import (
	"context"
	"testing"
	"time"
)

func TestCleanParagraphs(t *testing.T) {
	rawHTML := `<p> 第一段文本内容。<br/>换行内容。 </p><p>第二段包含&nbsp;空格&#34;双引号&#34;与零宽字符\u200b。</p>`
	paras := CleanParagraphs(rawHTML)
	if len(paras) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(paras))
	}
	if paras[0] != "第一段文本内容。\n换行内容。" {
		t.Errorf("unexpected paragraph 0: %q", paras[0])
	}
}

func TestFetchMetadataReal(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with "起点" (short demo book)
	bookID := "7307273223832355390"
	meta, err := client.FetchMetadata(ctx, bookID)
	if err != nil {
		t.Fatalf("FetchMetadata failed: %v", err)
	}

	if meta.Title != "起点" {
		t.Errorf("expected Title '起点', got %q", meta.Title)
	}
	if meta.Author != "小临尘" {
		t.Errorf("expected Author '小临尘', got %q", meta.Author)
	}
	if len(meta.FirstChapters) == 0 {
		t.Errorf("expected at least 1 first chapter from landing page, got %d", len(meta.FirstChapters))
	}

	t.Logf("Metadata: Title=%s, Author=%s, Score=%s, ChapterCount=%d",
		meta.Title, meta.Author, meta.Score, meta.ChapterCount)
}

func TestFetchChapterListReal(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with "惊鸿"
	bookID := "6883748331202284558"
	chapters, err := client.FetchChapterList(ctx, bookID)
	if err != nil {
		t.Fatalf("FetchChapterList failed: %v", err)
	}

	if len(chapters) == 0 {
		t.Fatalf("expected chapters > 0, got 0")
	}

	first := chapters[0]
	if first.ItemID == "" || first.Title == "" {
		t.Fatalf("invalid first chapter: %+v", first)
	}

	t.Logf("Fetched %d chapters. Chapter 1: %s (%s)", len(chapters), first.Title, first.ItemID)
}

func TestFetchChapterAndDecryptReal(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Test fetching chapter 7307267487517655614
	itemID := "7307267487517655614"
	content, err := client.FetchChapter(ctx, itemID)
	if err != nil {
		t.Fatalf("FetchChapter failed: %v", err)
	}

	if len(content.Paragraphs) == 0 {
		t.Fatalf("expected non-empty paragraphs")
	}

	fullText := ""
	for _, p := range content.Paragraphs {
		fullText += p + "\n"
	}

	// Verify no remaining PUA characters
	remaining := RemainingPUA(fullText)
	if len(remaining) > 0 {
		t.Errorf("detected %d remaining PUA characters in decrypted text: %v", len(remaining), remaining)
	}

	t.Logf("Decrypted chapter title: %s, paragraphs: %d, word count: %d",
		content.Title, len(content.Paragraphs), content.WordCount)
	t.Logf("First paragraph preview: %s", content.Paragraphs[0])
}

func TestFetchRankListReal(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Fetch main rank
	ranks, err := client.FetchRankList(ctx, "")
	if err != nil {
		t.Fatalf("FetchRankList failed: %v", err)
	}

	if len(ranks) == 0 {
		t.Fatalf("expected at least 1 rank book, got 0")
	}

	first := ranks[0]
	if first.BookID == "" || first.Title == "" {
		t.Fatalf("invalid rank book: %+v", first)
	}

	// Verify no remaining PUA in rank titles
	if rem := RemainingPUA(first.Title); len(rem) > 0 {
		t.Errorf("detected PUA in rank book title %q: %v", first.Title, rem)
	}

	t.Logf("Fetched %d rank books. #1: 《%s》 by %s (%s, %s)",
		len(ranks), first.Title, first.Author, first.ReadCount, first.WordCount)
}
