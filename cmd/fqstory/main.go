package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"fqstory/internal/crawler"
	"fqstory/internal/storage"
	"fqstory/internal/ui"
)

func main() {
	bookInput := flag.String("book", "", "可选：番茄小说链接、分享文案或书籍ID（带此参数时直接打开或添加）")
	previewChapter := flag.Int("preview", 0, "可选：快速在命令行打印指定章节正文并退出（非交互模式）")
	flag.Parse()

	// Initialize local SQLite storage
	store, err := storage.New("")
	if err != nil {
		fmt.Printf("❌ 初始化本地数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	client := crawler.NewClient()

	// 1. Non-interactive CLI preview mode
	if *previewChapter > 0 && *bookInput != "" {
		runCLIPreview(client, *bookInput, *previewChapter)
		return
	}

	// 2. Interactive TUI Mode (Default)
	model := ui.NewModel(store, client)

	// If -book is provided, pre-load and save the book before launching TUI
	if *bookInput != "" {
		bookID, err := crawler.ResolveBookID(*bookInput)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			meta, err := client.FetchMetadata(ctx, bookID)
			cancel()
			if err == nil && meta != nil {
				_ = store.SaveBook(&storage.Book{
					BookID:        bookID,
					Title:         meta.Title,
					Author:        meta.Author,
					Score:         meta.Score,
					CoverURL:      meta.CoverURL,
					Intro:         meta.Intro,
					ChapterCount:  meta.ChapterCount,
					LastReadIndex: 1,
					LastReadTitle: "第1章",
				})
			}
		}
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("❌ 运行 TUI 失败: %v\n", err)
		os.Exit(1)
	}
}

func runCLIPreview(client *crawler.Client, bookInput string, chapterNum int) {
	fmt.Println("🔍 正在解析书籍标识...")
	bookID, err := crawler.ResolveBookID(bookInput)
	if err != nil {
		fmt.Printf("❌ 解析失败: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	meta, err := client.FetchMetadata(ctx, bookID)
	if err != nil {
		fmt.Printf("❌ 获取元数据失败: %v\n", err)
		os.Exit(1)
	}

	chapters, err := client.FetchChapterList(ctx, bookID)
	if err != nil {
		fmt.Printf("❌ 获取目录失败: %v\n", err)
		os.Exit(1)
	}

	if chapterNum > len(chapters) {
		fmt.Printf("❌ 章节超出范围 (总共 %d 章)\n", len(chapters))
		os.Exit(1)
	}

	target := chapters[chapterNum-1]
	fmt.Printf("📖 《%s》- 第 %d 章: %s (ID: %s)\n\n", meta.Title, chapterNum, target.Title, target.ItemID)

	content, err := client.FetchChapterWithFallback(ctx, meta, target.ItemID, chapterNum)
	if err != nil {
		fmt.Printf("❌ 获取正文失败: %v\n", err)
		os.Exit(1)
	}

	for _, p := range content.Paragraphs {
		fmt.Printf("    %s\n\n", p)
	}
}
