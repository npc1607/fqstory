package ui

import (
	"fqstory/internal/crawler"
	"fqstory/internal/storage"
)

type booksLoadedMsg struct {
	books []*storage.Book
	err   error
}

type bookAddedMsg struct {
	book *storage.Book
	err  error
}

type chaptersLoadedMsg struct {
	bookID   string
	chapters []crawler.Chapter
	err      error
}

type chapterLoadedMsg struct {
	content *crawler.ChapterContent
	index   int
	err     error
}

type rankLoadedMsg struct {
	categoryIndex int
	books         []crawler.RankBook
	err           error
}

type batchDownloadMsg struct {
	current int
	total   int
	done    bool
	err     error
}

type autoScrollTickMsg struct{}

type errMsg struct {
	err error
}
