package crawler

// BookMetadata contains descriptive information about a novel.
type BookMetadata struct {
	BookID           string   `json:"book_id"`
	Title            string   `json:"title"`
	Author           string   `json:"author"`
	Score            string   `json:"score"`
	Intro            string   `json:"intro"`
	WordCount        int64    `json:"word_count"`
	ChapterCount     int      `json:"chapter_count"`
	SerialStatus     string   `json:"serial_status"`
	CoverURL         string   `json:"cover_url"`
	CategoryTags     []string `json:"category_tags"`
	ReadingCountText string   `json:"reading_count_text"`
	FirstChapters    []string `json:"first_chapters"` // Up to 10 plaintext chapters from landing page
}

// Chapter represents an item in the book's table of contents.
type Chapter struct {
	Index      int    `json:"index"`
	ItemID     string `json:"item_id"`
	Title      string `json:"title"`
	VolumeName string `json:"volume_name"`
	Locked     bool   `json:"locked"`
}

// ChapterContent contains the decrypted paragraphs of a single chapter.
type ChapterContent struct {
	ItemID     string   `json:"item_id"`
	Title      string   `json:"title"`
	Paragraphs []string `json:"paragraphs"`
	PreItemID  string   `json:"pre_item_id"`
	NextItemID string   `json:"next_item_id"`
	WordCount  int      `json:"word_count"`
	Source     string   `json:"source"` // "landing" or "reader"
}

// RankBook represents a novel item in Fanqie rankings.
type RankBook struct {
	Rank        int    `json:"rank"`
	BookID      string `json:"book_id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	ReadCount   string `json:"read_count"`
	WordCount   string `json:"word_count"`
	Intro       string `json:"intro"`
	LastChapter string `json:"last_chapter"`
}

// RankCategory represents a ranking genre tab.
type RankCategory struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

