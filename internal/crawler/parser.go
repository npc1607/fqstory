package crawler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	// urlRe matches http/https URLs in text
	urlRe = regexp.MustCompile(`https?://[^\s<>"'）)】\]，,。；;、]+`)

	// urlBookIDRe extracts book_id parameter from URL query
	urlBookIDRe = regexp.MustCompile(`(?i)(?:book_id|bookId|bookid|series_id)=(\d{10,25})`)

	// pathBookIDRe extracts book_id from URL path like /page/123456 or /reader/123456
	pathBookIDRe = regexp.MustCompile(`(?i)fanqienovel\.com/(?:page|reader)/(\d{10,25})`)

	// bareIDRe matches pure digits
	bareIDRe = regexp.MustCompile(`^\s*(\d{10,25})\s*$`)

	// htmlBookIDRes searches for book_id hints in HTML body
	htmlBookIDRes = []*regexp.Regexp{
		regexp.MustCompile(`"book_id"\s*:\s*"?(\d{10,25})`),
		regexp.MustCompile(`"bookId"\s*:\s*"?(\d{10,25})`),
		regexp.MustCompile(`book_id=(\d{10,25})`),
		regexp.MustCompile(`/page/(\d{10,25})`),
	}
)

// ExtractBookIDFromText inspects text without making network calls.
func ExtractBookIDFromText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	if m := bareIDRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	if m := pathBookIDRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	if m := urlBookIDRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	return ""
}

// ExtractFirstURL extracts the first http(s) URL in text.
func ExtractFirstURL(text string) string {
	return urlRe.FindString(text)
}

// ResolveBookID resolves user input (pure ID, share text, short link, web URL) to book_id.
func ResolveBookID(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("输入为空，请输入番茄小说链接、分享文案或书籍ID")
	}

	// 1. Quick check without network
	if id := ExtractBookIDFromText(input); id != "" {
		return id, nil
	}

	// 2. Extract URL
	url := ExtractFirstURL(input)
	if url == "" {
		return "", errors.New("未识别到有效的小说ID或网页链接")
	}

	// 3. Follow redirect
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("重定向次数过多")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Referer", "https://changdunovel.com/")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求链接失败: %w", err)
	}
	defer resp.Body.Close()

	// Check final URL
	finalURL := resp.Request.URL.String()
	if id := ExtractBookIDFromText(finalURL); id != "" {
		return id, nil
	}

	// Check HTML body
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*512))
	if err == nil && len(bodyBytes) > 0 {
		bodyStr := string(bodyBytes)
		for _, reg := range htmlBookIDRes {
			if m := reg.FindStringSubmatch(bodyStr); len(m) > 1 {
				return m[1], nil
			}
		}
	}

	return "", fmt.Errorf("无法从链接 %s 中提取书籍 ID", url)
}
