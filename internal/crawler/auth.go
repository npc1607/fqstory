package crawler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	passportAID = "2503"
	qrNextURL   = "https://fanqienovel.com/"
	qrGetURL    = "https://fanqienovel.com/passport/web/get_qrcode/"
	qrCheckURL  = "https://fanqienovel.com/passport/web/check_qrconnect/"
	userInfoURL = "https://fanqienovel.com/api/user/info/v2"
)

// AuthTicket represents a pending login session with QR code.
type AuthTicket struct {
	Token      string `json:"token"`
	QRCodeURL  string `json:"qrcode_url"`
	QRArt      string `json:"qr_art"`
	ImagePath  string `json:"image_path"`
	ExpireTime int64  `json:"expire_time"`
}

// UserInfo represents logged-in Fanqie user details.
type UserInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Desc   string `json:"desc"`
	IsVIP  bool   `json:"is_vip"`
}

func passportParams() url.Values {
	v := url.Values{}
	v.Set("aid", passportAID)
	v.Set("app_name", "novelapp")
	v.Set("version_code", "57700")
	v.Set("device_platform", "web")
	v.Set("channel", "novel")
	v.Set("sdk_version", "1.6.1")
	v.Set("passport_sdk_version", "2.0.0")
	v.Set("new_user", "0")
	return v
}

// GetLoginQRCode requests official Douyin/Fanqie QR code login credentials.
func (c *Client) GetLoginQRCode(ctx context.Context) (*AuthTicket, error) {
	params := passportParams()
	params.Set("next", qrNextURL)

	reqURL := fmt.Sprintf("%s?%s", qrGetURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建二维码请求失败: %w", err)
	}
	req.Header.Set("User-Agent", desktopUA)
	req.Header.Set("Referer", "https://fanqienovel.com/")
	req.Header.Set("Origin", "https://fanqienovel.com")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求二维码接口失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取二维码数据失败: %w", err)
	}

	var raw struct {
		Data struct {
			ErrorCode      int    `json:"error_code"`
			Description    string `json:"description"`
			Token          string `json:"token"`
			QRCodeBase64   string `json:"qrcode"`
			QRCodeIndexURL string `json:"qrcode_index_url"`
			ExpireTime     int64  `json:"expire_time"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("解析二维码响应失败: %w", err)
	}

	d := raw.Data
	if d.ErrorCode != 0 || d.Token == "" {
		return nil, fmt.Errorf("获取二维码失败: %s (code=%d)", d.Description, d.ErrorCode)
	}

	qrTargetURL := d.QRCodeIndexURL
	if qrTargetURL == "" {
		qrTargetURL = fmt.Sprintf("https://fanqienovel.com/login?token=%s", d.Token)
	}

	// Generate small terminal-friendly QR Art
	q, err := qrcode.New(qrTargetURL, qrcode.Medium)
	var qrArt string
	if err == nil {
		qrArt = q.ToSmallString(false)
	}

	// Save PNG file to ~/.config/fqstory/login_qr.png
	var imagePath string
	if d.QRCodeBase64 != "" {
		pngBytes, err := base64.StdEncoding.DecodeString(d.QRCodeBase64)
		if err == nil {
			home, _ := os.UserHomeDir()
			if home != "" {
				dir := filepath.Join(home, ".config", "fqstory")
				_ = os.MkdirAll(dir, 0755)
				filePath := filepath.Join(dir, "login_qr.png")
				if err := os.WriteFile(filePath, pngBytes, 0644); err == nil {
					imagePath = filePath
				}
			}
		}
	}

	return &AuthTicket{
		Token:      d.Token,
		QRCodeURL:  qrTargetURL,
		QRArt:      qrArt,
		ImagePath:  imagePath,
		ExpireTime: d.ExpireTime,
	}, nil
}

// CheckQRConnect polls status of the QR code scan.
// Possible statuses: "new" (waiting), "scanned" (waiting phone confirmation), "confirmed" (success).
func (c *Client) CheckQRConnect(ctx context.Context, token string) (status string, cookieStr string, err error) {
	if token == "" {
		return "", "", errors.New("token 不能为空")
	}

	params := passportParams()
	params.Set("next", qrNextURL)
	params.Set("token", token)

	reqURL := fmt.Sprintf("%s?%s", qrCheckURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("创建轮询请求失败: %w", err)
	}
	req.Header.Set("User-Agent", desktopUA)
	req.Header.Set("Referer", "https://fanqienovel.com/")
	req.Header.Set("Origin", "https://fanqienovel.com")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("轮询二维码状态失败: %w", err)
	}
	defer resp.Body.Close()

	// Extract cookies if any
	var cookies []string
	for _, ck := range resp.Cookies() {
		cookies = append(cookies, fmt.Sprintf("%s=%s", ck.Name, ck.Value))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取状态响应失败: %w", err)
	}

	var raw struct {
		Data struct {
			ErrorCode   int    `json:"error_code"`
			Status      string `json:"status"`
			RedirectURL string `json:"redirect_url"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return "", "", fmt.Errorf("解析轮询响应失败: %w", err)
	}

	st := raw.Data.Status
	if raw.Data.RedirectURL != "" || strings.Contains(strings.ToLower(st), "confirm") || strings.Contains(strings.ToLower(st), "success") {
		st = "confirmed"
		// If cookies were returned, concatenate them
		if len(cookies) > 0 {
			cookieStr = strings.Join(cookies, "; ")
		}
	}

	return st, cookieStr, nil
}

// GetUserInfo validates cookie and fetches account details.
func (c *Client) GetUserInfo(ctx context.Context, cookieStr string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建用户请求失败: %w", err)
	}
	req.Header.Set("User-Agent", desktopUA)
	req.Header.Set("Referer", "https://fanqienovel.com/")
	if cookieStr != "" {
		req.Header.Set("Cookie", cookieStr)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取用户数据失败: %w", err)
	}

	var raw struct {
		Code int `json:"code"`
		Data struct {
			ID     interface{} `json:"id"`
			Name   string      `json:"name"`
			Avatar string      `json:"avatar"`
			Desc   string      `json:"desc"`
			IsVIP  bool        `json:"isVip"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("解析用户信息 JSON 失败: %w", err)
	}

	idStr := fmt.Sprintf("%v", raw.Data.ID)
	if idStr == "" || idStr == "<nil>" || idStr == "0" || idStr == "1" {
		return nil, errors.New("当前未登录或 Cookie 已过期")
	}

	return &UserInfo{
		ID:     idStr,
		Name:   raw.Data.Name,
		Avatar: raw.Data.Avatar,
		Desc:   raw.Data.Desc,
		IsVIP:  raw.Data.IsVIP,
	}, nil
}

// SetCookie dynamically updates the client's authentication cookie.
func (c *Client) SetCookie(cookie string) {
	c.cookie = strings.TrimSpace(cookie)
}

// Cookie returns the current authentication cookie.
func (c *Client) Cookie() string {
	return c.cookie
}
