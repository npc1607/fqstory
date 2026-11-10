package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fqstory/internal/crawler"

	_ "modernc.org/sqlite"
)

// Book represents a novel stored in user's bookshelf.
type Book struct {
	BookID          string    `json:"book_id"`
	Title           string    `json:"title"`
	Author          string    `json:"author"`
	Score           string    `json:"score"`
	CoverURL        string    `json:"cover_url"`
	Intro           string    `json:"intro"`
	ChapterCount    int       `json:"chapter_count"`
	LastReadIndex   int       `json:"last_read_index"`
	LastReadTitle   string    `json:"last_read_title"`
	LastReadOffset  int       `json:"last_read_offset"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Storage manages local persistence in SQLite.
type Storage struct {
	db *sql.DB
}

// DefaultDBPath returns the standard user config path for the database.
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "fqstory.db"
	}
	dir := filepath.Join(home, ".config", "fqstory")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "fqstory.db")
}

// New creates and initializes a Storage instance.
func New(dbPath string) (*Storage, error) {
	if dbPath == "" {
		dbPath = DefaultDBPath()
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	s := &Storage{db: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化数据库表结构失败: %w", err)
	}

	return s, nil
}

func (s *Storage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS books (
		book_id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		author TEXT DEFAULT '',
		score TEXT DEFAULT '',
		cover_url TEXT DEFAULT '',
		intro TEXT DEFAULT '',
		chapter_count INTEGER DEFAULT 0,
		last_read_index INTEGER DEFAULT 1,
		last_read_title TEXT DEFAULT '',
		last_read_offset INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chapters (
		book_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		item_id TEXT NOT NULL,
		title TEXT NOT NULL,
		content_json TEXT NOT NULL,
		word_count INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (book_id, chapter_index)
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *Storage) Close() error {
	return s.db.Close()
}

// SaveBook adds or updates a book in the bookshelf.
func (s *Storage) SaveBook(b *Book) error {
	if b == nil || b.BookID == "" {
		return fmt.Errorf("无效的书籍数据")
	}

	query := `
	INSERT INTO books (
		book_id, title, author, score, cover_url, intro,
		chapter_count, last_read_index, last_read_title, last_read_offset, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(book_id) DO UPDATE SET
		title = excluded.title,
		author = excluded.author,
		score = excluded.score,
		cover_url = excluded.cover_url,
		intro = excluded.intro,
		chapter_count = excluded.chapter_count,
		updated_at = excluded.updated_at;
	`
	now := time.Now()
	if b.LastReadIndex <= 0 {
		b.LastReadIndex = 1
	}

	_, err := s.db.Exec(query,
		b.BookID, b.Title, b.Author, b.Score, b.CoverURL, b.Intro,
		b.ChapterCount, b.LastReadIndex, b.LastReadTitle, b.LastReadOffset, now,
	)
	return err
}

// UpdateProgress updates the reading progress for a book.
func (s *Storage) UpdateProgress(bookID string, chapterIndex int, chapterTitle string, offset int) error {
	query := `
	UPDATE books SET
		last_read_index = ?,
		last_read_title = ?,
		last_read_offset = ?,
		updated_at = ?
	WHERE book_id = ?;
	`
	_, err := s.db.Exec(query, chapterIndex, chapterTitle, offset, time.Now(), bookID)
	return err
}

// GetBook fetches a book by bookID.
func (s *Storage) GetBook(bookID string) (*Book, error) {
	query := `
	SELECT book_id, title, author, score, cover_url, intro,
	       chapter_count, last_read_index, last_read_title, last_read_offset, updated_at
	FROM books WHERE book_id = ?;
	`
	row := s.db.QueryRow(query, bookID)

	var b Book
	var updatedAt time.Time
	err := row.Scan(
		&b.BookID, &b.Title, &b.Author, &b.Score, &b.CoverURL, &b.Intro,
		&b.ChapterCount, &b.LastReadIndex, &b.LastReadTitle, &b.LastReadOffset, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	b.UpdatedAt = updatedAt
	return &b, nil
}

// ListBooks lists all books sorted by most recently updated.
func (s *Storage) ListBooks() ([]*Book, error) {
	query := `
	SELECT book_id, title, author, score, cover_url, intro,
	       chapter_count, last_read_index, last_read_title, last_read_offset, updated_at
	FROM books ORDER BY updated_at DESC;
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Book
	for rows.Next() {
		var b Book
		var updatedAt time.Time
		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Author, &b.Score, &b.CoverURL, &b.Intro,
			&b.ChapterCount, &b.LastReadIndex, &b.LastReadTitle, &b.LastReadOffset, &updatedAt,
		); err != nil {
			return nil, err
		}
		b.UpdatedAt = updatedAt
		list = append(list, &b)
	}
	return list, nil
}

// DeleteBook removes a book and its cached chapters.
func (s *Storage) DeleteBook(bookID string) error {
	_, _ = s.db.Exec("DELETE FROM chapters WHERE book_id = ?", bookID)
	_, err := s.db.Exec("DELETE FROM books WHERE book_id = ?", bookID)
	return err
}

// SaveChapter caches decrypted chapter content.
func (s *Storage) SaveChapter(bookID string, index int, itemID, title string, paragraphs []string, wordCount int) error {
	data, err := json.Marshal(paragraphs)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO chapters (book_id, chapter_index, item_id, title, content_json, word_count)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(book_id, chapter_index) DO UPDATE SET
		item_id = excluded.item_id,
		title = excluded.title,
		content_json = excluded.content_json,
		word_count = excluded.word_count;
	`
	_, err = s.db.Exec(query, bookID, index, itemID, title, string(data), wordCount)
	return err
}

// GetChapter returns cached chapter content if available.
func (s *Storage) GetChapter(bookID string, index int) (*crawler.ChapterContent, error) {
	query := `
	SELECT item_id, title, content_json, word_count
	FROM chapters WHERE book_id = ? AND chapter_index = ?;
	`
	row := s.db.QueryRow(query, bookID, index)

	var itemID, title, contentJSON string
	var wordCount int
	err := row.Scan(&itemID, &title, &contentJSON, &wordCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var paragraphs []string
	if err := json.Unmarshal([]byte(contentJSON), &paragraphs); err != nil {
		return nil, err
	}

	return &crawler.ChapterContent{
		ItemID:     itemID,
		Title:      title,
		Paragraphs: paragraphs,
		WordCount:  wordCount,
		Source:     "cache",
	}, nil
}

// HasChapter returns true if the specified chapter is already cached locally.
func (s *Storage) HasChapter(bookID string, index int) bool {
	var count int
	err := s.db.QueryRow("SELECT COUNT(1) FROM chapters WHERE book_id = ? AND chapter_index = ?", bookID, index).Scan(&count)
	return err == nil && count > 0
}
