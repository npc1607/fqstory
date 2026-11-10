package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 15 * time.Second

	landingAPIURL   = "https://api.fqnovel.com/novel_ug/share/landing_page"
	directoryAPIURL = "https://fanqienovel.com/api/reader/directory/detail"
	readerPageURL   = "https://fanqienovel.com/reader/%s"

	desktopUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	mobileUA  = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1"
)

var (
	tagRe  = regexp.MustCompile(`<[^>]+>`)
	pTagRe = regexp.MustCompile(`(?i)<p(?:\s[^>]*)?>(.*?)</p>`)
	wsRe   = regexp.MustCompile(`[ \t\f\v\x{00a0}]+`)
)

// extractInitialStateJSON extracts the balanced JSON object assigned to window.__INITIAL_STATE__.
func extractInitialStateJSON(pageHTML string) (string, error) {
	idx := strings.Index(pageHTML, "window.__INITIAL_STATE__")
	if idx == -1 {
		return "", errors.New("页面中未找到 window.__INITIAL_STATE__")
	}
	braceStart := strings.Index(pageHTML[idx:], "{")
	if braceStart == -1 {
		return "", errors.New("未找到 JSON 起始花括号")
	}
	start := idx + braceStart
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(pageHTML); i++ {
		ch := pageHTML[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return pageHTML[start : i+1], nil
			}
		}
	}
	return "", errors.New("window.__INITIAL_STATE__ JSON 花括号未闭合")
}

// Client handles communication with Fanqie Novel endpoints.
type Client struct {
	httpClient *http.Client
	cookie     string
}

// ClientOption defines functional options for Client.
type ClientOption func(*Client)

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithCookie sets custom Fanqie user cookie.
func WithCookie(cookie string) ClientOption {
	return func(c *Client) {
		c.cookie = strings.TrimSpace(cookie)
	}
}

// NewClient creates a new Fanqie crawler client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CleanParagraphs parses HTML fragments into clean paragraph slices.
func CleanParagraphs(htmlFragment string) []string {
	if htmlFragment == "" {
		return nil
	}

	matches := pTagRe.FindAllStringSubmatch(htmlFragment, -1)
	var paragraphs []string
	if len(matches) > 0 {
		for _, m := range matches {
			if len(m) > 1 {
				cleaned := cleanSingleParagraph(m[1])
				if cleaned != "" {
					paragraphs = append(paragraphs, cleaned)
				}
			}
		}
		if len(paragraphs) > 0 {
			return paragraphs
		}
	}

	// Fallback: split by lines if no <p> tags found
	lines := strings.Split(htmlFragment, "\n")
	for _, line := range lines {
		cleaned := cleanSingleParagraph(line)
		if cleaned != "" {
			paragraphs = append(paragraphs, cleaned)
		}
	}
	return paragraphs
}

func cleanSingleParagraph(fragment string) string {
	fragment = strings.ReplaceAll(fragment, "<br/>", "\n")
	fragment = strings.ReplaceAll(fragment, "<br>", "\n")
	fragment = tagRe.ReplaceAllString(fragment, "")
	text := html.UnescapeString(fragment)

	// Strip zero-width & non-standard whitespaces
	text = strings.ReplaceAll(text, "\u200b", "")
	text = strings.ReplaceAll(text, "\ufeff", "")
	text = strings.ReplaceAll(text, "\u3000", " ")
	text = wsRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// FetchMetadata retrieves book metadata and first plaintext chapters from landing page.
func (c *Client) FetchMetadata(ctx context.Context, bookID string) (*BookMetadata, error) {
	if bookID == "" {
		return nil, errors.New("book_id 不能为空")
	}

	params := url.Values{}
	params.Set("aid", "1967")
	params.Set("series_id", "0")
	params.Set("performance_optimization", "1")
	params.Set("share_type", "11")
	params.Set("book_id", bookID)

	reqURL := fmt.Sprintf("%s?%s", landingAPIURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", mobileUA)
	req.Header.Set("Referer", "https://changdunovel.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求落地页失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("落地页接口响应状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var raw struct {
		ErrNo   int    `json:"err_no"`
		ErrTips string `json:"err_tips"`
		Data    struct {
			BookData struct {
				Title            string   `json:"title"`
				Author           struct {
					Name string `json:"name"`
				} `json:"author"`
				Score            interface{} `json:"score"`
				Intro            string      `json:"intro"`
				WordCount        interface{} `json:"word_count"`
				SerialCount      interface{} `json:"serial_count"`
				CreationStatus   int         `json:"creation_status"`
				CoverURL         string      `json:"cover_url"`
				CategoryTags     []string    `json:"category_tags"`
				ReadingCountText string      `json:"reading_count_text"`
				FirstChapters    []string    `json:"first_chapters_content"`
			} `json:"book_data"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析书籍元数据 JSON 失败: %w", err)
	}

	if raw.ErrNo != 0 {
		return nil, fmt.Errorf("获取书籍失败: %s (err_no=%d)", raw.ErrTips, raw.ErrNo)
	}

	bd := raw.Data.BookData
	if bd.Title == "" {
		return nil, errors.New("书籍信息为空或已下架")
	}

	// Parse word count
	var wordCount int64
	switch v := bd.WordCount.(type) {
	case float64:
		wordCount = int64(v)
	case string:
		wordCount, _ = strconv.ParseInt(v, 10, 64)
	}

	// Parse chapter count
	var chapterCount int
	switch v := bd.SerialCount.(type) {
	case float64:
		chapterCount = int(v)
	case string:
		chapterCount, _ = strconv.Atoi(v)
	}

	scoreStr := fmt.Sprintf("%v", bd.Score)
	if scoreStr == "<nil>" || scoreStr == "0" {
		scoreStr = "暂无"
	}

	statusStr := "连载中"
	if bd.CreationStatus == 0 {
		statusStr = "已完结"
	}

	return &BookMetadata{
		BookID:           bookID,
		Title:            bd.Title,
		Author:           bd.Author.Name,
		Score:            scoreStr,
		Intro:            strings.TrimSpace(bd.Intro),
		WordCount:        wordCount,
		ChapterCount:     chapterCount,
		SerialStatus:     statusStr,
		CoverURL:         bd.CoverURL,
		CategoryTags:     bd.CategoryTags,
		ReadingCountText: bd.ReadingCountText,
		FirstChapters:    bd.FirstChapters,
	}, nil
}

// FetchChapterList retrieves the full table of contents for a book.
func (c *Client) FetchChapterList(ctx context.Context, bookID string) ([]Chapter, error) {
	if bookID == "" {
		return nil, errors.New("book_id 不能为空")
	}

	reqURL := fmt.Sprintf("%s?bookId=%s", directoryAPIURL, url.QueryEscape(bookID))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建目录请求失败: %w", err)
	}

	req.Header.Set("User-Agent", desktopUA)
	req.Header.Set("Referer", "https://fanqienovel.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求目录失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("目录接口返回状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取目录响应失败: %w", err)
	}

	var raw struct {
		Code int `json:"code"`
		Data struct {
			AllItemIDs            []interface{} `json:"allItemIds"`
			ChapterListWithVolume [][]struct {
				ItemID            interface{} `json:"itemId"`
				Title             string      `json:"title"`
				VolumeName        string      `json:"volume_name"`
				NeedPay           int         `json:"needPay"`
				IsChapterLock     bool        `json:"isChapterLock"`
				IsPaidPublication bool        `json:"isPaidPublication"`
			} `json:"chapterListWithVolume"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析目录 JSON 失败: %w", err)
	}

	var chapters []Chapter
	for _, vol := range raw.Data.ChapterListWithVolume {
		for _, item := range vol {
			itemIDStr := fmt.Sprintf("%v", item.ItemID)
			if itemIDStr == "" || itemIDStr == "<nil>" {
				continue
			}
			locked := item.IsChapterLock || item.NeedPay > 0 || item.IsPaidPublication
			chapters = append(chapters, Chapter{
				Index:      len(chapters) + 1,
				ItemID:     itemIDStr,
				Title:      strings.TrimSpace(item.Title),
				VolumeName: item.VolumeName,
				Locked:     locked,
			})
		}
	}

	if len(chapters) == 0 && len(raw.Data.AllItemIDs) > 0 {
		for i, id := range raw.Data.AllItemIDs {
			chapters = append(chapters, Chapter{
				Index:  i + 1,
				ItemID: fmt.Sprintf("%v", id),
				Title:  fmt.Sprintf("第%d章", i+1),
				Locked: false,
			})
		}
	}

	return chapters, nil
}

// FetchChapter fetches a single chapter, cleans the HTML, and decrypts the text using fontMap.
func (c *Client) FetchChapter(ctx context.Context, itemID string) (*ChapterContent, error) {
	if itemID == "" {
		return nil, errors.New("item_id 不能为空")
	}

	reqURL := fmt.Sprintf(readerPageURL, itemID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建章节请求失败: %w", err)
	}

	req.Header.Set("User-Agent", desktopUA)
	req.Header.Set("Referer", "https://fanqienovel.com/")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求章节页面失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("章节未找到或未上架 (404)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("章节接口响应状态码: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取章节页面失败: %w", err)
	}
	pageHTML := string(bodyBytes)

	// Check if this page requires VIP login
	if strings.Contains(pageHTML, "muye-to-vip") || strings.Contains(pageHTML, "扫码下载APP免费读") {
		// Even for VIP preview, we still try to extract available preview paragraphs
	}

	jsonStr, err := extractInitialStateJSON(pageHTML)
	if err != nil {
		return nil, fmt.Errorf("提取章节状态数据失败: %w", err)
	}

	var state struct {
		Reader struct {
			ChapterData struct {
				Title             string `json:"title"`
				Content           string `json:"content"`
				PreItemID         string `json:"preItemId"`
				NextItemID        string `json:"nextItemId"`
				ChapterWordNumber string `json:"chapterWordNumber"`
			} `json:"chapterData"`
		} `json:"reader"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		return nil, fmt.Errorf("解析 __INITIAL_STATE__ 失败: %w", err)
	}

	rawContent := state.Reader.ChapterData.Content
	rawParas := CleanParagraphs(rawContent)
	if len(rawParas) == 0 {
		return nil, errors.New("章节正文为空")
	}

	// Decrypt each paragraph through our fontMap
	decryptedParas := make([]string, len(rawParas))
	for i, p := range rawParas {
		decryptedParas[i] = DecryptText(p)
	}

	title := state.Reader.ChapterData.Title
	if title == "" {
		title = fmt.Sprintf("章节 %s", itemID)
	}

	wordCount, _ := strconv.Atoi(state.Reader.ChapterData.ChapterWordNumber)
	if wordCount == 0 {
		for _, p := range decryptedParas {
			wordCount += len([]rune(p))
		}
	}

	return &ChapterContent{
		ItemID:     itemID,
		Title:      title,
		Paragraphs: decryptedParas,
		PreItemID:  state.Reader.ChapterData.PreItemID,
		NextItemID: state.Reader.ChapterData.NextItemID,
		WordCount:  wordCount,
		Source:     "reader",
	}, nil
}

// FetchChapterWithFallback tries reading from landing page first (if index <= 10 and available),
// otherwise falls back to online Reader API with font decryption.
func (c *Client) FetchChapterWithFallback(ctx context.Context, meta *BookMetadata, itemID string, chapterIndex int) (*ChapterContent, error) {
	// If chapter index <= len(meta.FirstChapters), landing page offers direct plaintext!
	if meta != nil && chapterIndex > 0 && chapterIndex <= len(meta.FirstChapters) {
		rawHTML := meta.FirstChapters[chapterIndex-1]
		paras := CleanParagraphs(rawHTML)
		if len(paras) > 0 {
			var wc int
			for _, p := range paras {
				wc += len([]rune(p))
			}
			return &ChapterContent{
				ItemID:     itemID,
				Title:      fmt.Sprintf("第%d章", chapterIndex),
				Paragraphs: paras,
				WordCount:  wc,
				Source:     "landing",
			}, nil
		}
	}

	return c.FetchChapter(ctx, itemID)
}

// Predefined popular rank categories
var DefaultRankCategories = []RankCategory{
	{Key: "", Name: "热门总榜"},
	{Key: "1_2_258", Name: "传统玄幻"},
	{Key: "1_2_1140", Name: "东方仙侠"},
	{Key: "1_2_1014", Name: "都市高武"},
	{Key: "1_2_261", Name: "都市日常"},
	{Key: "1_2_8", Name: "科幻末世"},
	{Key: "0_2_1139", Name: "古风世情"},
	{Key: "0_2_248", Name: "玄幻言情"},
}

// FetchRankList retrieves ranking books for a specific category key.
func (c *Client) FetchRankList(ctx context.Context, categoryKey string) ([]RankBook, error) {
	reqURL := "https://fanqienovel.com/rank"
	if categoryKey != "" {
		reqURL = fmt.Sprintf("https://fanqienovel.com/rank/%s", categoryKey)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建排行榜请求失败: %w", err)
	}
	req.Header.Set("User-Agent", desktopUA)
	req.Header.Set("Referer", "https://fanqienovel.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求排行榜失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("排行榜接口返回状态码: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取排行榜页面失败: %w", err)
	}

	jsonStr, err := extractInitialStateJSON(string(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("提取排行榜状态失败: %w", err)
	}

	var state struct {
		Rank struct {
			BookList []struct {
				BookID           interface{} `json:"bookId"`
				BookName         string      `json:"bookName"`
				Author           string      `json:"author"`
				ReadCount        interface{} `json:"read_count"`
				WordNumber       interface{} `json:"wordNumber"`
				Abstract         string      `json:"abstract"`
				LastChapterTitle string      `json:"lastChapterTitle"`
				CurrentPos       int         `json:"currentPos"`
			} `json:"book_list"`
		} `json:"rank"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		return nil, fmt.Errorf("解析排行榜 JSON 失败: %w", err)
	}

	var books []RankBook
	for i, item := range state.Rank.BookList {
		bookIDStr := fmt.Sprintf("%v", item.BookID)
		if bookIDStr == "" || bookIDStr == "<nil>" {
			continue
		}

		pos := item.CurrentPos
		if pos <= 0 {
			pos = i + 1
		}

		title := DecryptText(item.BookName)
		author := DecryptText(item.Author)
		intro := DecryptText(item.Abstract)

		readCountStr := fmt.Sprintf("%v", item.ReadCount)
		if rc, err := strconv.ParseInt(readCountStr, 10, 64); err == nil && rc > 0 {
			if rc >= 10000 {
				readCountStr = fmt.Sprintf("%.1f万在读", float64(rc)/10000.0)
			} else {
				readCountStr = fmt.Sprintf("%d在读", rc)
			}
		}

		wordCountStr := fmt.Sprintf("%v", item.WordNumber)
		if wc, err := strconv.ParseInt(wordCountStr, 10, 64); err == nil && wc > 0 {
			if wc >= 10000 {
				wordCountStr = fmt.Sprintf("%.1f万字", float64(wc)/10000.0)
			} else {
				wordCountStr = fmt.Sprintf("%d字", wc)
			}
		}

		books = append(books, RankBook{
			Rank:        pos,
			BookID:      bookIDStr,
			Title:       title,
			Author:      author,
			ReadCount:   readCountStr,
			WordCount:   wordCountStr,
			Intro:       intro,
			LastChapter: DecryptText(item.LastChapterTitle),
		})
	}

	return books, nil
}
