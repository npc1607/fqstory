package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"fqstory/internal/crawler"
	"fqstory/internal/masquerade"
	"fqstory/internal/storage"
)

// ScreenState manages the active view.
type ScreenState int

const (
	StateShelf ScreenState = iota
	StateAddBook
	StateBookDetail
	StateRank
	StateLogin
	StateToc
	StateReader
	StateBoss
	StateLoading
)

// Model is the main Bubble Tea model for fqstory.
type Model struct {
	state     ScreenState
	prevState ScreenState
	store     *storage.Storage
	client    *crawler.Client

	// Bookshelf state
	books       []*storage.Book
	shelfCursor int

	// Current active book
	currentBook    *storage.Book
	chapters       []crawler.Chapter
	tocCursor      int
	currChapterIdx int // 1-based index
	currChapter    *crawler.ChapterContent

	// Rankings state
	rankCategories  []crawler.RankCategory
	rankCategoryIdx int
	rankBooks       []crawler.RankBook
	rankCursor      int

	// TOC Search & Filtering
	tocSearchInput   textinput.Model
	tocSearching     bool
	filteredChapters []crawler.Chapter

	// Stealth mode
	stealthMode      masquerade.Mode
	autoScroll       bool
	miniTickerOffset int
	fakeScreenType   int // 0: GoTest, 1: GitStatus, 2: Docker, 3: Htop

	// Auth / Login state
	currentUser  *crawler.UserInfo
	authTicket   *crawler.AuthTicket
	loginMode    int // 0: QR Scan, 1: Paste Cookie
	cookieInput  textinput.Model
	loginPolling bool
	loginStatus  string

	// Batch Download
	batchDownloading bool
	batchProgress    string

	// Components
	viewport  viewport.Model
	textInput textinput.Model
	spinner   spinner.Model

	// UI Dimensions
	width  int
	height int

	// Status & Error messages
	statusMsg string
	err       error
}

// NewModel creates an initialized TUI Model.
func NewModel(store *storage.Storage, client *crawler.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "粘贴番茄链接、分享文案或书籍ID (例如 6883748331202284558)"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60

	tocInput := textinput.New()
	tocInput.Placeholder = "输入章节名称或数字快速跳转 (如 100 或 剑仙)"
	tocInput.CharLimit = 64
	tocInput.Width = 40

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	cookieIn := textinput.New()
	cookieIn.Placeholder = "粘贴浏览器 Cookie (如 sessionid=...)"
	cookieIn.CharLimit = 1024
	cookieIn.Width = 60

	return Model{
		state:          StateShelf,
		store:          store,
		client:         client,
		textInput:      ti,
		tocSearchInput: tocInput,
		cookieInput:    cookieIn,
		spinner:        sp,
		rankCategories: crawler.DefaultRankCategories,
		stealthMode:    masquerade.ModeClassic,
		autoScroll:     false,
		width:          80,
		height:         24,
	}
}

// Init starts initial commands.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadBooksCmd(),
		m.loadPersistedUserCmd(),
		m.spinner.Tick,
	)
}

func (m Model) loadPersistedUserCmd() tea.Cmd {
	return func() tea.Msg {
		cookie, err := m.store.GetConfig("auth_cookie")
		if err == nil && cookie != "" {
			m.client.SetCookie(cookie)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			u, err := m.client.GetUserInfo(ctx, cookie)
			if err == nil && u != nil {
				return userLoadedMsg{user: u, cookie: cookie}
			}
		}
		return userLoadedMsg{user: nil, cookie: ""}
	}
}

func (m Model) loadBooksCmd() tea.Cmd {
	return func() tea.Msg {
		books, err := m.store.ListBooks()
		return booksLoadedMsg{books: books, err: err}
	}
}

func (m Model) loadChapterContentCmd(bookID string, index int, itemID string) tea.Cmd {
	return func() tea.Msg {
		// 1. Try local SQLite cache first
		cached, err := m.store.GetChapter(bookID, index)
		if err == nil && cached != nil && len(cached.Paragraphs) > 0 {
			return chapterLoadedMsg{content: cached, index: index, err: nil}
		}

		// 2. Fetch from network
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		content, err := m.client.FetchChapter(ctx, itemID)
		if err != nil {
			return chapterLoadedMsg{content: nil, index: index, err: err}
		}

		// 3. Cache to SQLite
		_ = m.store.SaveChapter(bookID, index, itemID, content.Title, content.Paragraphs, content.WordCount)
		return chapterLoadedMsg{content: content, index: index, err: nil}
	}
}

func (m Model) loadChaptersListCmd(bookID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		chapters, err := m.client.FetchChapterList(ctx, bookID)
		return chaptersLoadedMsg{bookID: bookID, chapters: chapters, err: err}
	}
}

func (m Model) prefetchNextChapters(bookID string, currentIdx int, chapters []crawler.Chapter) {
	go func() {
		maxPrefetch := 5
		for i := 1; i <= maxPrefetch; i++ {
			targetIdx := currentIdx + i
			if targetIdx > len(chapters) {
				break
			}
			if m.store.HasChapter(bookID, targetIdx) {
				continue
			}

			targetChapter := chapters[targetIdx-1]
			if targetChapter.ItemID == "" {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			content, err := m.client.FetchChapter(ctx, targetChapter.ItemID)
			cancel()
			if err == nil && content != nil && len(content.Paragraphs) > 0 {
				_ = m.store.SaveChapter(bookID, targetIdx, targetChapter.ItemID, content.Title, content.Paragraphs, content.WordCount)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}()
}

func (m Model) startBatchDownloadCmd(bookID string, startIdx int, count int) tea.Cmd {
	return func() tea.Msg {
		total := count
		doneCount := 0
		for i := 0; i < count; i++ {
			targetIdx := startIdx + i
			if targetIdx > len(m.chapters) {
				break
			}
			if !m.store.HasChapter(bookID, targetIdx) {
				ch := m.chapters[targetIdx-1]
				if ch.ItemID != "" {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					content, err := m.client.FetchChapter(ctx, ch.ItemID)
					cancel()
					if err == nil && content != nil && len(content.Paragraphs) > 0 {
						_ = m.store.SaveChapter(bookID, targetIdx, ch.ItemID, content.Title, content.Paragraphs, content.WordCount)
					}
					time.Sleep(350 * time.Millisecond)
				}
			}
			doneCount++
		}
		return batchDownloadMsg{current: doneCount, total: total, done: true, err: nil}
	}
}

func (m Model) loadRankCmd(catIdx int) tea.Cmd {
	return func() tea.Msg {
		if catIdx < 0 || catIdx >= len(m.rankCategories) {
			catIdx = 0
		}
		cat := m.rankCategories[catIdx]
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		books, err := m.client.FetchRankList(ctx, cat.Key)
		return rankLoadedMsg{categoryIndex: catIdx, books: books, err: err}
	}
}

func (m Model) startQRLoginCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		ticket, err := m.client.GetLoginQRCode(ctx)
		return qrCodeReceivedMsg{ticket: ticket, err: err}
	}
}

func qrPollTicker() tea.Cmd {
	return tea.Tick(2000*time.Millisecond, func(t time.Time) tea.Msg {
		return qrPollTickMsg{}
	})
}

func (m Model) verifyAndSaveCookieCmd(cookieStr string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		user, err := m.client.GetUserInfo(ctx, cookieStr)
		if err != nil {
			return errMsg{err: fmt.Errorf("Cookie 验证失败或已过期: %w", err)}
		}
		_ = m.store.SaveConfig("auth_cookie", cookieStr)
		m.client.SetCookie(cookieStr)
		return loginSuccessMsg{user: user, cookie: cookieStr}
	}
}

func autoScrollTicker() tea.Cmd {
	return tea.Tick(2000*time.Millisecond, func(t time.Time) tea.Msg {
		return autoScrollTickMsg{}
	})
}

// Update handles Bubble Tea messages and keybindings.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = max(20, msg.Width-4)
		m.viewport.Height = max(5, msg.Height-4)
		if m.currChapter != nil {
			m.renderCurrentChapterToViewport()
		}

	case booksLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("加载书架失败: %v", msg.err)
		} else {
			m.books = msg.books
			if m.shelfCursor >= len(m.books) {
				m.shelfCursor = max(0, len(m.books)-1)
			}
		}

	case chaptersLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("获取目录失败: %v", msg.err)
			m.state = StateShelf
		} else {
			m.chapters = msg.chapters
			m.filteredChapters = msg.chapters
			if m.state == StateLoading && m.currentBook != nil {
				targetIdx := m.currentBook.LastReadIndex
				if targetIdx <= 0 || targetIdx > len(m.chapters) {
					targetIdx = 1
				}
				m.currChapterIdx = targetIdx
				targetChapter := m.chapters[targetIdx-1]
				m.state = StateLoading
				return m, m.loadChapterContentCmd(m.currentBook.BookID, targetIdx, targetChapter.ItemID)
			}
		}

	case chapterLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("加载章节失败: %v", msg.err)
			m.state = StateShelf
		} else {
			m.currChapter = msg.content
			m.currChapterIdx = msg.index
			m.miniTickerOffset = 0
			m.state = StateReader
			m.renderCurrentChapterToViewport()

			if m.currentBook != nil && m.currentBook.LastReadIndex == msg.index && m.currentBook.LastReadOffset > 0 {
				m.viewport.SetYOffset(m.currentBook.LastReadOffset)
			} else {
				m.viewport.GotoTop()
			}

			if m.currentBook != nil {
				_ = m.store.UpdateProgress(m.currentBook.BookID, msg.index, msg.content.Title, m.viewport.YOffset)
				m.currentBook.LastReadIndex = msg.index
				m.currentBook.LastReadTitle = msg.content.Title
			}

			m.prefetchNextChapters(m.currentBook.BookID, msg.index, m.chapters)
		}

	case bookAddedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("添加书籍失败: %v", msg.err)
			m.state = StateShelf
		} else {
			m.currentBook = msg.book
			m.state = StateLoading
			m.statusMsg = fmt.Sprintf("正在拉取《%s》目录...", msg.book.Title)
			return m, m.loadChaptersListCmd(msg.book.BookID)
		}

	case batchDownloadMsg:
		m.batchDownloading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("离线缓存异常: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("已成功离线预载 %d 章正文！", msg.current)
		}

	case rankLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("获取排行榜失败: %v", msg.err)
		} else {
			m.rankBooks = msg.books
			m.rankCategoryIdx = msg.categoryIndex
			m.rankCursor = 0
			m.statusMsg = fmt.Sprintf("排行榜加载成功，共 %d 本热门小说", len(msg.books))
		}

	case userLoadedMsg:
		m.currentUser = msg.user

	case qrCodeReceivedMsg:
		if msg.err != nil {
			m.loginStatus = fmt.Sprintf("获取二维码失败: %v", msg.err)
			m.loginPolling = false
		} else {
			m.authTicket = msg.ticket
			m.loginPolling = true
			m.loginStatus = "请使用【抖音 / 番茄小说 APP】扫描下方二维码并在手机上确认"
			return m, qrPollTicker()
		}

	case qrPollTickMsg:
		if m.state == StateLogin && m.loginPolling && m.authTicket != nil {
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				status, cookieStr, err := m.client.CheckQRConnect(ctx, m.authTicket.Token)
				if err == nil {
					if status == "confirmed" {
						_ = m.store.SaveConfig("auth_cookie", cookieStr)
						m.client.SetCookie(cookieStr)
						u, _ := m.client.GetUserInfo(ctx, cookieStr)
						if u == nil {
							u = &crawler.UserInfo{Name: "番茄书友", IsVIP: true}
						}
						return loginSuccessMsg{user: u, cookie: cookieStr}
					}
					if strings.Contains(status, "scan") {
						return qrCodeReceivedMsg{ticket: m.authTicket, err: nil}
					}
				}
				return qrPollTickMsg{}
			}
		}

	case loginSuccessMsg:
		m.currentUser = msg.user
		m.loginPolling = false
		m.state = StateShelf
		vipTag := ""
		if msg.user != nil && msg.user.IsVIP {
			vipTag = " [VIP 会员]"
		}
		userName := "用户"
		if msg.user != nil && msg.user.Name != "" {
			userName = msg.user.Name
		}
		m.statusMsg = fmt.Sprintf("🎉 登录成功！欢迎回来，%s%s", userName, vipTag)
		return m, m.loadBooksCmd()

	case errMsg:
		m.statusMsg = fmt.Sprintf("❌ %v", msg.err)
		if m.state == StateLogin {
			m.loginStatus = fmt.Sprintf("❌ %v", msg.err)
		}

	case autoScrollTickMsg:
		if m.state == StateReader && m.autoScroll {
			if m.stealthMode == masquerade.ModeStatusLine {
				m.miniTickerOffset += 18
			} else {
				m.viewport.LineDown(1)
				if m.currentBook != nil {
					_ = m.store.UpdateProgress(m.currentBook.BookID, m.currChapterIdx, m.currChapter.Title, m.viewport.YOffset)
				}
			}
			return m, autoScrollTicker()
		}

	case tea.KeyMsg:
		// 1. In Boss screen: any key seamlessly restores previous view
		if m.state == StateBoss {
			m.state = m.prevState
			return m, nil
		}

		// 2. Boss Key triggering:
		// 'b' is the universal emergency boss key (available in Shelf, Reader, Rank, Toc, BookDetail, Login)
		// 'esc' also triggers boss key specifically when inside Reader view
		isInputting := (m.state == StateAddBook) || (m.state == StateLogin && m.loginMode == 1) || (m.state == StateToc && m.tocSearching)
		if (msg.String() == "b" && !isInputting) || (msg.String() == "esc" && m.state == StateReader) {
			m.prevState = m.state
			m.state = StateBoss
			return m, nil
		}

		// Route per active screen
		switch m.state {
		case StateShelf:
			return m.updateShelf(msg)
		case StateAddBook:
			return m.updateAddBook(msg)
		case StateBookDetail:
			return m.updateBookDetail(msg)
		case StateRank:
			return m.updateRank(msg)
		case StateLogin:
			return m.updateLogin(msg)
		case StateToc:
			return m.updateToc(msg)
		case StateReader:
			return m.updateReader(msg)
		}
	}

	// Update active input components
	if m.state == StateAddBook {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.state == StateLogin && m.loginMode == 1 {
		var cmd tea.Cmd
		m.cookieInput, cmd = m.cookieInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.state == StateToc && m.tocSearching {
		var cmd tea.Cmd
		m.tocSearchInput, cmd = m.tocSearchInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.state == StateLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateShelf(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if len(m.books) > 0 {
			m.shelfCursor = (m.shelfCursor + 1) % len(m.books)
		}
	case "k", "up":
		if len(m.books) > 0 {
			m.shelfCursor = (m.shelfCursor - 1 + len(m.books)) % len(m.books)
		}
	case "i":
		if len(m.books) > 0 && m.shelfCursor < len(m.books) {
			m.currentBook = m.books[m.shelfCursor]
			m.state = StateBookDetail
			return m, nil
		}
	case "r":
		m.state = StateRank
		m.statusMsg = "正在拉取番茄排行榜..."
		return m, m.loadRankCmd(m.rankCategoryIdx)
	case "L", "l":
		if m.currentUser != nil {
			_ = m.store.DeleteConfig("auth_cookie")
			m.client.SetCookie("")
			m.currentUser = nil
			m.statusMsg = "已成功退出登录并清除本地凭证"
			return m, nil
		}
		m.state = StateLogin
		m.loginMode = 0
		m.loginPolling = true
		m.loginStatus = "正在向番茄官网请求登录二维码..."
		return m, m.startQRLoginCmd()
	case "a", "/":
		m.state = StateAddBook
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink
	case "d":
		if len(m.books) > 0 && m.shelfCursor < len(m.books) {
			toDel := m.books[m.shelfCursor]
			_ = m.store.DeleteBook(toDel.BookID)
			return m, m.loadBooksCmd()
		}
	case "enter":
		if len(m.books) > 0 && m.shelfCursor < len(m.books) {
			m.currentBook = m.books[m.shelfCursor]
			m.state = StateLoading
			m.statusMsg = fmt.Sprintf("正在打开《%s》...", m.currentBook.Title)
			return m, m.loadChaptersListCmd(m.currentBook.BookID)
		}
	}
	return m, nil
}

func (m Model) updateBookDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = StateShelf
		return m, nil
	case "enter":
		// Start reading this book
		m.state = StateLoading
		m.statusMsg = fmt.Sprintf("正在打开《%s》...", m.currentBook.Title)
		return m, m.loadChaptersListCmd(m.currentBook.BookID)
	case "D":
		// Trigger offline batch download of next 20 chapters
		if m.currentBook != nil && len(m.chapters) > 0 {
			m.batchDownloading = true
			m.statusMsg = "正在后台批量缓存接下来 20 章..."
			return m, m.startBatchDownloadCmd(m.currentBook.BookID, m.currentBook.LastReadIndex, 20)
		}
	}
	return m, nil
}

func (m Model) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateShelf
		m.loginPolling = false
		return m, m.loadBooksCmd()

	case "o":
		if m.authTicket != nil && m.authTicket.ImagePath != "" {
			_ = crawler.OpenSystemFile(m.authTicket.ImagePath)
			m.loginStatus = "已在外部窗口打开二维码图片，请使用手机扫描"
			return m, nil
		}

	case "c":
		m.loginMode = 1
		m.loginPolling = false
		m.cookieInput.SetValue("")
		m.cookieInput.Focus()
		return m, textinput.Blink

	case "s":
		m.loginMode = 0
		m.loginPolling = true
		m.loginStatus = "正在刷新登录二维码..."
		return m, m.startQRLoginCmd()

	case "ctrl+v":
		if m.loginMode == 1 {
			text, err := clipboard.ReadAll()
			if err == nil && text != "" {
				m.cookieInput.SetValue(strings.TrimSpace(text))
				m.cookieInput.SetCursor(len(m.cookieInput.Value()))
			}
			return m, nil
		}

	case "enter":
		if m.loginMode == 1 {
			cookieStr := strings.TrimSpace(m.cookieInput.Value())
			if cookieStr == "" {
				return m, nil
			}
			m.loginStatus = "正在校验 Cookie 有效性..."
			return m, m.verifyAndSaveCookieCmd(cookieStr)
		} else if m.authTicket != nil && m.authTicket.ImagePath != "" {
			_ = crawler.OpenSystemFile(m.authTicket.ImagePath)
			m.loginStatus = "已在外部窗口打开二维码图片，请使用手机扫描"
			return m, nil
		}
	}

	if m.loginMode == 1 {
		var cmd tea.Cmd
		m.cookieInput, cmd = m.cookieInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateRank(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = StateShelf
		return m, m.loadBooksCmd()

	case "tab", "l", "right":
		m.rankCategoryIdx = (m.rankCategoryIdx + 1) % len(m.rankCategories)
		m.rankBooks = nil
		m.statusMsg = fmt.Sprintf("正在加载【%s】榜单...", m.rankCategories[m.rankCategoryIdx].Name)
		return m, m.loadRankCmd(m.rankCategoryIdx)

	case "shift+tab", "h", "left":
		m.rankCategoryIdx = (m.rankCategoryIdx - 1 + len(m.rankCategories)) % len(m.rankCategories)
		m.rankBooks = nil
		m.statusMsg = fmt.Sprintf("正在加载【%s】榜单...", m.rankCategories[m.rankCategoryIdx].Name)
		return m, m.loadRankCmd(m.rankCategoryIdx)

	case "j", "down":
		if len(m.rankBooks) > 0 {
			m.rankCursor = min(len(m.rankBooks)-1, m.rankCursor+1)
		}
	case "k", "up":
		if len(m.rankBooks) > 0 {
			m.rankCursor = max(0, m.rankCursor-1)
		}

	case "i":
		if len(m.rankBooks) > 0 && m.rankCursor < len(m.rankBooks) {
			target := m.rankBooks[m.rankCursor]
			m.currentBook = &storage.Book{
				BookID:        target.BookID,
				Title:         target.Title,
				Author:        target.Author,
				Score:         target.ReadCount,
				Intro:         target.Intro,
				LastReadTitle: target.LastChapter,
			}
			m.state = StateBookDetail
			return m, nil
		}

	case "a":
		if len(m.rankBooks) > 0 && m.rankCursor < len(m.rankBooks) {
			target := m.rankBooks[m.rankCursor]
			b := &storage.Book{
				BookID:        target.BookID,
				Title:         target.Title,
				Author:        target.Author,
				Score:         target.ReadCount,
				Intro:         target.Intro,
				LastReadIndex: 1,
				LastReadTitle: "第1章",
			}
			_ = m.store.SaveBook(b)
			m.statusMsg = fmt.Sprintf("已成功将《%s》加入书架！", target.Title)
			return m, nil
		}

	case "enter":
		if len(m.rankBooks) > 0 && m.rankCursor < len(m.rankBooks) {
			target := m.rankBooks[m.rankCursor]
			b := &storage.Book{
				BookID:        target.BookID,
				Title:         target.Title,
				Author:        target.Author,
				Score:         target.ReadCount,
				Intro:         target.Intro,
				LastReadIndex: 1,
				LastReadTitle: "第1章",
			}
			_ = m.store.SaveBook(b)
			m.currentBook = b
			m.state = StateLoading
			m.statusMsg = fmt.Sprintf("正在载入《%s》...", b.Title)
			return m, m.loadChaptersListCmd(b.BookID)
		}
	}
	return m, nil
}

func (m Model) updateAddBook(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateShelf
		return m, nil
	case "ctrl+v":
		text, err := clipboard.ReadAll()
		if err == nil && text != "" {
			curr := m.textInput.Value()
			m.textInput.SetValue(curr + strings.TrimSpace(text))
			m.textInput.SetCursor(len(m.textInput.Value()))
		}
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.textInput.Value())
		if input == "" {
			return m, nil
		}
		m.state = StateLoading
		m.statusMsg = "正在解析书籍信息..."

		return m, func() tea.Msg {
			bookID, err := crawler.ResolveBookID(input)
			if err != nil {
				return bookAddedMsg{book: nil, err: err}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			meta, err := m.client.FetchMetadata(ctx, bookID)
			if err != nil {
				return bookAddedMsg{book: nil, err: err}
			}

			b := &storage.Book{
				BookID:        bookID,
				Title:         meta.Title,
				Author:        meta.Author,
				Score:         meta.Score,
				CoverURL:      meta.CoverURL,
				Intro:         meta.Intro,
				ChapterCount:  meta.ChapterCount,
				LastReadIndex: 1,
				LastReadTitle: "第1章",
			}
			if err := m.store.SaveBook(b); err != nil {
				return bookAddedMsg{book: nil, err: err}
			}
			return bookAddedMsg{book: b, err: nil}
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) updateToc(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If in search input mode
	if m.tocSearching {
		switch msg.String() {
		case "esc":
			m.tocSearching = false
			m.filteredChapters = m.chapters
			m.tocCursor = max(0, min(m.tocCursor, len(m.filteredChapters)-1))
			return m, nil
		case "enter":
			m.tocSearching = false
			if len(m.filteredChapters) > 0 && m.tocCursor < len(m.filteredChapters) {
				target := m.filteredChapters[m.tocCursor]
				m.state = StateLoading
				m.statusMsg = fmt.Sprintf("正在拉取 %s...", target.Title)
				return m, m.loadChapterContentCmd(m.currentBook.BookID, target.Index, target.ItemID)
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.tocSearchInput, cmd = m.tocSearchInput.Update(msg)
		query := strings.TrimSpace(m.tocSearchInput.Value())
		if query == "" {
			m.filteredChapters = m.chapters
		} else {
			var matched []crawler.Chapter
			queryLower := strings.ToLower(query)
			for _, ch := range m.chapters {
				if strings.Contains(strings.ToLower(ch.Title), queryLower) ||
					strings.Contains(strconv.Itoa(ch.Index), query) {
					matched = append(matched, ch)
				}
			}
			m.filteredChapters = matched
		}
		m.tocCursor = 0
		return m, cmd
	}

	// Normal TOC navigation
	switch msg.String() {
	case "esc", "q":
		if m.currChapter != nil {
			m.state = StateReader
		} else {
			m.state = StateShelf
		}
		return m, nil
	case "/":
		m.tocSearching = true
		m.tocSearchInput.SetValue("")
		m.tocSearchInput.Focus()
		return m, textinput.Blink
	case "j", "down":
		if len(m.filteredChapters) > 0 {
			m.tocCursor = min(len(m.filteredChapters)-1, m.tocCursor+1)
		}
	case "k", "up":
		if len(m.filteredChapters) > 0 {
			m.tocCursor = max(0, m.tocCursor-1)
		}
	case "g":
		m.tocCursor = 0
	case "G":
		m.tocCursor = max(0, len(m.filteredChapters)-1)
	case "enter":
		if len(m.filteredChapters) > 0 && m.tocCursor < len(m.filteredChapters) {
			target := m.filteredChapters[m.tocCursor]
			m.state = StateLoading
			m.statusMsg = fmt.Sprintf("正在拉取 %s...", target.Title)
			return m, m.loadChapterContentCmd(m.currentBook.BookID, target.Index, target.ItemID)
		}
	}
	return m, nil
}

func (m Model) updateReader(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		if m.currentBook != nil && m.currChapter != nil {
			_ = m.store.UpdateProgress(m.currentBook.BookID, m.currChapterIdx, m.currChapter.Title, m.viewport.YOffset)
		}
		m.state = StateShelf
		return m, m.loadBooksCmd()

	case "m":
		m.stealthMode = m.stealthMode.Next()
		m.renderCurrentChapterToViewport()
		m.statusMsg = fmt.Sprintf("已切换至: %s", m.stealthMode.Name())
		return m, nil

	case "c":
		// Cycle fake background screen for statusline / boss mode
		m.fakeScreenType = (m.fakeScreenType + 1) % 4
		names := []string{"Go Test", "Git Status", "Docker PS", "Htop"}
		m.statusMsg = fmt.Sprintf("伪装背景已切至: %s", names[m.fakeScreenType])
		return m, nil

	case "a":
		m.autoScroll = !m.autoScroll
		if m.autoScroll {
			m.statusMsg = "已开启自动滚屏"
			return m, autoScrollTicker()
		}
		m.statusMsg = "已停止自动滚屏"
		return m, nil

	case "t":
		m.state = StateToc
		m.filteredChapters = m.chapters
		m.tocCursor = max(0, m.currChapterIdx-1)
		return m, nil

	case "D":
		// Batch prefetch next 20 chapters
		if m.currentBook != nil && len(m.chapters) > 0 {
			m.batchDownloading = true
			m.statusMsg = "正在后台离线缓存接下来 20 章..."
			return m, m.startBatchDownloadCmd(m.currentBook.BookID, m.currChapterIdx+1, 20)
		}

	case "n":
		if m.currChapterIdx < len(m.chapters) {
			nextIdx := m.currChapterIdx + 1
			target := m.chapters[nextIdx-1]
			m.state = StateLoading
			m.statusMsg = fmt.Sprintf("正在翻到下一章: %s...", target.Title)
			return m, m.loadChapterContentCmd(m.currentBook.BookID, nextIdx, target.ItemID)
		}
		m.statusMsg = "已经是最新章节"

	case "p":
		if m.currChapterIdx > 1 {
			prevIdx := m.currChapterIdx - 1
			target := m.chapters[prevIdx-1]
			m.state = StateLoading
			m.statusMsg = fmt.Sprintf("正在回到上一章: %s...", target.Title)
			return m, m.loadChapterContentCmd(m.currentBook.BookID, prevIdx, target.ItemID)
		}
		m.statusMsg = "已经是第一章"

	// Navigation
	case "l", "right":
		if m.stealthMode == masquerade.ModeStatusLine {
			m.miniTickerOffset += 25
			return m, nil
		}
	case "h", "left":
		if m.stealthMode == masquerade.ModeStatusLine {
			m.miniTickerOffset = max(0, m.miniTickerOffset-25)
			return m, nil
		}
	case "space":
		if m.stealthMode == masquerade.ModeStatusLine {
			m.miniTickerOffset += 30
			return m, nil
		}
		m.viewport.ViewDown()
	case "j", "down":
		m.viewport.LineDown(1)
	case "k", "up":
		m.viewport.LineUp(1)
	case "d":
		m.viewport.HalfViewDown()
	case "u":
		m.viewport.HalfViewUp()
	}

	return m, nil
}

func (m *Model) renderCurrentChapterToViewport() {
	if m.currChapter == nil {
		return
	}

	maxWidth := m.viewport.Width
	if maxWidth <= 0 {
		maxWidth = max(40, m.width-4)
	}

	var formatted string
	switch m.stealthMode {
	case masquerade.ModeLog:
		formatted = masquerade.FormatLog(m.currChapter.Paragraphs, time.Now(), maxWidth)
	case masquerade.ModeCode:
		formatted = masquerade.FormatCode(m.currChapter.Paragraphs, "order", maxWidth)
	case masquerade.ModeClassic:
		formatted = masquerade.FormatClassic(m.currChapter.Paragraphs, maxWidth)
	default:
		formatted = masquerade.FormatClassic(m.currChapter.Paragraphs, maxWidth)
	}

	m.viewport.SetContent(formatted)
}

func (m Model) getFakeBackground() string {
	switch m.fakeScreenType {
	case 1:
		return masquerade.GenerateFakeGitScreen()
	case 2:
		return masquerade.GenerateFakeDockerScreen()
	case 3:
		return masquerade.GenerateFakeHtopScreen()
	default:
		return masquerade.GenerateFakeGoTestScreen()
	}
}

// View handles screen rendering.
func (m Model) View() string {
	switch m.state {
	case StateShelf:
		return m.viewShelf()
	case StateBookDetail:
		return m.viewBookDetail()
	case StateRank:
		return m.viewRank()
	case StateLogin:
		return m.viewLogin()
	case StateAddBook:
		return m.viewAddBook()
	case StateToc:
		return m.viewToc()
	case StateReader:
		return m.viewReader()
	case StateBoss:
		return m.viewBoss()
	case StateLoading:
		return m.viewLoading()
	}
	return ""
}

func (m Model) viewHeader() string {
	brand := badgeBrand.Render("🍅 FQSTORY")
	title := headerTitleStyle.Render("番茄小说 · 极速摸鱼阅读器")

	var userBadge string
	if m.currentUser != nil {
		vipTag := ""
		if m.currentUser.IsVIP {
			vipTag = " [VIP]"
		}
		userBadge = lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Background(colorPurple).Padding(0, 1).MarginLeft(1).Render("👤 " + m.currentUser.Name + vipTag)
	} else {
		userBadge = headerPillStyle.Render("👤 未登录 (按 L 登录)")
	}

	bookCount := headerPillStyle.Render(fmt.Sprintf("📚 书库: %d 本", len(m.books)))
	stealthPill := headerPillStyle.Render("🛡️ 摸鱼守护: 已就绪")

	left := lipgloss.JoinHorizontal(lipgloss.Center, brand, title)
	right := lipgloss.JoinHorizontal(lipgloss.Center, userBadge, bookCount, stealthPill)

	gapWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gapWidth < 1 {
		gapWidth = 1
	}
	gap := strings.Repeat(" ", gapWidth)

	return lipgloss.JoinHorizontal(lipgloss.Center, left, gap, right) + "\n"
}

func (m Model) viewFooter() string {
	loginDesc := "登录"
	if m.currentUser != nil {
		loginDesc = "登出"
	}

	keys := []struct {
		key  string
		desc string
	}{
		{"Enter", "立即开读"},
		{"r", "热门榜单"},
		{"a", "导入新书"},
		{"L", loginDesc},
		{"i", "完整详情"},
		{"d", "移出书架"},
		{"q", "退出"},
	}

	var parts []string
	for _, k := range keys {
		part := lipgloss.JoinHorizontal(lipgloss.Center, keyCapKey.Render(k.key), " ", keyCapDesc.Render(k.desc))
		parts = append(parts, part)
	}
	dock := strings.Join(parts, " ")

	bossBadge := bossKeyBadge.Render("⚡ [b/Esc] 0.1秒老板键")

	gapWidth := m.width - lipgloss.Width(dock) - lipgloss.Width(bossBadge) - 4
	if gapWidth < 1 {
		gapWidth = 1
	}
	gap := strings.Repeat(" ", gapWidth)

	return lipgloss.JoinHorizontal(lipgloss.Center, dock, gap, bossBadge)
}

func (m Model) viewShelf() string {
	var b strings.Builder
	b.WriteString(m.viewHeader())
	b.WriteString("\n")

	if len(m.books) == 0 {
		emptyCard := cardBoxStyle.Width(min(70, m.width-6)).Render(
			panelTitleStyle.Render("✨ 欢迎来到 fqstory 终端书架") + "\n\n" +
				lipgloss.NewStyle().Foreground(colorWhite).Render("  书架目前还是空的，你可以通过以下方式开启摸鱼：") + "\n\n" +
				lipgloss.NewStyle().Foreground(colorGold).Render("  • [ r ] 浏览番茄小说热门排行榜，一键挑选当红爆款") + "\n" +
				lipgloss.NewStyle().Foreground(colorCyan).Render("  • [ a ] 或 [ / ] 粘贴番茄小说链接、分享文案或书籍 ID 导入") + "\n" +
				lipgloss.NewStyle().Foreground(colorMint).Render("  • [ Ctrl+V ] 在输入框中一键读取系统剪贴板") + "\n\n" +
				lipgloss.NewStyle().Foreground(colorDarkGray).Render("  提示：阅读时随时按 [ b ] 或 [ Esc ] 瞬间进入全屏工作伪装屏！"),
		)
		b.WriteString(emptyCard)
		b.WriteString("\n\n")
	} else {
		// If wide screen, render side-by-side: left list, right preview card
		if m.width >= 85 {
			leftWidth := int(float64(m.width-8) * 0.56)
			rightWidth := (m.width - 8) - leftWidth

			// 1. Left: Book list panel
			var leftContent strings.Builder
			leftContent.WriteString(panelTitleStyle.Render("📖 我的书架 (My Bookshelf)"))
			leftContent.WriteString("\n")

			for i, book := range m.books {
				ratio := 0.0
				if book.ChapterCount > 0 {
					ratio = float64(book.LastReadIndex) / float64(book.ChapterCount)
				}
				pBar := renderProgressBar(ratio, 8)

				if i == m.shelfCursor {
					line := fmt.Sprintf("▶ 📚 《%s》\n     作者: %s  %s", book.Title, book.Author, pBar)
					leftContent.WriteString(cardActiveBoxStyle.Width(leftWidth - 4).Render(line))
				} else {
					line := fmt.Sprintf("  📖 《%s》  作者: %s  %s", book.Title, book.Author, pBar)
					leftContent.WriteString(cardBoxStyle.Width(leftWidth - 4).Render(line))
				}
				leftContent.WriteString("\n")
			}

			leftPanel := cardBoxStyle.Width(leftWidth).Render(leftContent.String())

			// 2. Right: Quick preview panel for selected book
			selectedBook := m.books[m.shelfCursor]
			var rightContent strings.Builder
			rightContent.WriteString(panelTitleStyle.Render("🔍 选中古籍速览"))
			rightContent.WriteString("\n\n")

			titleHeader := lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render("《"+selectedBook.Title+"》"),
				" ",
				scoreBadgeStyle.Render("★ "+selectedBook.Score),
			)
			rightContent.WriteString(titleHeader)
			rightContent.WriteString("\n\n")

			metaLine := fmt.Sprintf("作  者: %s\n总章节: %d 章\n读  至: %s",
				selectedBook.Author, selectedBook.ChapterCount, selectedBook.LastReadTitle)
			rightContent.WriteString(lipgloss.NewStyle().Foreground(colorGray).Render(metaLine))
			rightContent.WriteString("\n\n")

			// Intro excerpt card
			introText := strings.ReplaceAll(selectedBook.Intro, "\n", " ")
			runes := []rune(introText)
			if len(runes) > 120 {
				introText = string(runes[:120]) + "..."
			}
			if introText == "" {
				introText = "暂无简介"
			}
			introCard := cardBoxStyle.Width(rightWidth - 6).Render("【内容简介】\n" + lipgloss.NewStyle().Foreground(colorDarkGray).Render(introText))
			rightContent.WriteString(introCard)

			rightPanel := cardBoxStyle.Width(rightWidth).Render(rightContent.String())

			// Combine horizontal
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel))
			b.WriteString("\n\n")
		} else {
			// Compact stacked list
			var listContent strings.Builder
			listContent.WriteString(panelTitleStyle.Render("📖 我的书架"))
			listContent.WriteString("\n")

			for i, book := range m.books {
				ratio := 0.0
				if book.ChapterCount > 0 {
					ratio = float64(book.LastReadIndex) / float64(book.ChapterCount)
				}
				pBar := renderProgressBar(ratio, 8)

				if i == m.shelfCursor {
					line := fmt.Sprintf("▶ 《%s》 - %s  %s", book.Title, book.Author, pBar)
					listContent.WriteString(bookRowSelected.Render(line))
				} else {
					line := fmt.Sprintf("  《%s》 - %s  %s", book.Title, book.Author, pBar)
					listContent.WriteString(bookRowNormal.Render(line))
				}
				listContent.WriteString("\n")
			}
			b.WriteString(cardBoxStyle.Width(m.width - 4).Render(listContent.String()))
			b.WriteString("\n\n")
		}
	}

	if m.statusMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Render("  ℹ " + m.statusMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(m.viewFooter())
	return b.String()
}

func (m Model) viewBookDetail() string {
	if m.currentBook == nil {
		return "书籍不存在"
	}
	b := m.currentBook
	pct := 0.0
	if b.ChapterCount > 0 {
		pct = float64(b.LastReadIndex) / float64(b.ChapterCount) * 100
	}

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render("《"+b.Title+"》"),
		" ",
		scoreBadgeStyle.Render("★ "+b.Score),
	)

	infoGrid := fmt.Sprintf(
		"作  者: %s\n"+
			"章节数: %d 章\n"+
			"读  至: %s  (已读 %.1f%%)",
		lipgloss.NewStyle().Foreground(colorCyan).Render(b.Author),
		b.ChapterCount,
		b.LastReadTitle, pct,
	)

	introCard := cardBoxStyle.Width(min(72, m.width-8)).Render(
		"【内容简介】\n" + lipgloss.NewStyle().Foreground(colorGray).Render(b.Intro),
	)

	keys := lipgloss.JoinHorizontal(lipgloss.Center,
		keyCapKey.Render("Enter"), " ", keyCapDesc.Render("开始阅读"),
		keyCapKey.Render("Shift+D"), " ", keyCapDesc.Render("离线缓存后20章"),
		keyCapKey.Render("Esc"), " ", keyCapDesc.Render("返回书架"),
	)

	content := fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s",
		header,
		infoGrid,
		introCard,
		keys,
	)
	return cardActiveBoxStyle.Width(min(76, m.width-4)).Render(content)
}

func (m Model) viewRank() string {
	var b strings.Builder

	// Top Banner
	brand := badgeBrand.Render("🔥 排行榜")
	title := headerTitleStyle.Render("番茄小说 · 热门排行榜")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, brand, title))
	b.WriteString("\n\n")

	// Render category tabs
	var tabParts []string
	for i, cat := range m.rankCategories {
		if i == m.rankCategoryIdx {
			tabParts = append(tabParts, tabActive.Render(cat.Name))
		} else {
			tabParts = append(tabParts, tabInactive.Render(cat.Name))
		}
	}
	b.WriteString(strings.Join(tabParts, " "))
	b.WriteString("\n\n")

	if len(m.rankBooks) == 0 {
		b.WriteString(cardBoxStyle.Width(m.width - 4).Render(dimStyle.Render("  正在拉取排行榜数据，请稍候...")))
		b.WriteString("\n\n")
	} else {
		var listContent strings.Builder
		for i, book := range m.rankBooks {
			prefix := "  "
			rowStyle := bookRowNormal
			if i == m.rankCursor {
				prefix = "▶ "
				rowStyle = bookRowSelected
			}

			// Crown badges for top 3
			rankTag := fmt.Sprintf("#%02d", book.Rank)
			if book.Rank == 1 {
				rankTag = scoreBadgeStyle.Render("#01 👑")
			} else if book.Rank == 2 {
				rankTag = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1e1e2e")).Background(colorWhite).Padding(0, 1).Render("#02 🥈")
			} else if book.Rank == 3 {
				rankTag = lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Background(lipgloss.Color("#cd7f32")).Padding(0, 1).Render("#03 🥉")
			}

			readBadge := readCountBadgeStyle.Render(book.ReadCount)
			wordBadge := tagStyle.Render(book.WordCount)

			line := fmt.Sprintf("%s%s 《%s》- %s  %s %s (最新: %s)",
				prefix, rankTag, book.Title, book.Author, readBadge, wordBadge, book.LastChapter)
			listContent.WriteString(rowStyle.Render(line))
			listContent.WriteString("\n")

			if i == m.rankCursor && book.Intro != "" {
				introSnippet := strings.ReplaceAll(book.Intro, "\n", " ")
				runes := []rune(introSnippet)
				if len(runes) > 75 {
					introSnippet = string(runes[:75]) + "..."
				}
				listContent.WriteString(lipgloss.NewStyle().Foreground(colorDarkGray).Render("     简介: " + introSnippet))
				listContent.WriteString("\n")
			}
		}
		b.WriteString(cardBoxStyle.Width(m.width - 4).Render(listContent.String()))
		b.WriteString("\n\n")
	}

	if m.statusMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Render("  ℹ " + m.statusMsg))
		b.WriteString("\n\n")
	}

	keys := lipgloss.JoinHorizontal(lipgloss.Center,
		keyCapKey.Render("Tab/←/→"), " ", keyCapDesc.Render("切换分类"),
		keyCapKey.Render("j/k"), " ", keyCapDesc.Render("选书"),
		keyCapKey.Render("Enter"), " ", keyCapDesc.Render("开读"),
		keyCapKey.Render("a"), " ", keyCapDesc.Render("加书架"),
		keyCapKey.Render("i"), " ", keyCapDesc.Render("详情"),
		keyCapKey.Render("Esc"), " ", keyCapDesc.Render("返回"),
	)
	b.WriteString(keys)
	return b.String()
}

func (m Model) viewLogin() string {
	if m.loginMode == 1 {
		// Paste Cookie mode
		header := headerTitleStyle.Render("🍪 粘贴 Cookie 登录")
		tip := dimStyle.Render("在浏览器打开 fanqienovel.com 登录后，从控制台/Network 中复制完整 Cookie：")
		keys := lipgloss.JoinHorizontal(lipgloss.Center,
			keyCapKey.Render("Enter"), " ", keyCapDesc.Render("验证并登录"),
			keyCapKey.Render("Ctrl+V"), " ", keyCapDesc.Render("一键粘贴"),
			keyCapKey.Render("s"), " ", keyCapDesc.Render("切换扫码"),
			keyCapKey.Render("Esc"), " ", keyCapDesc.Render("取消返回"),
		)
		statusLine := lipgloss.NewStyle().Foreground(colorCyan).Render(m.loginStatus)
		content := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s", header, tip, m.cookieInput.View(), statusLine, keys)
		return cardActiveBoxStyle.Width(min(72, m.width-4)).Render(content)
	}

	// QR Code Scan mode (Default)
	header := headerTitleStyle.Render("📱 抖音 / 番茄小说 App 扫码登录")

	qrLines := 30
	if m.authTicket != nil && m.authTicket.QRArtLines > 0 {
		qrLines = m.authTicket.QRArtLines
	}
	// Check if terminal window is tall and wide enough for full ASCII QR art
	canFitTerminal := (m.height >= qrLines+12) && (m.width >= 72)

	if canFitTerminal && m.authTicket != nil && m.authTicket.QRArt != "" {
		// Large screen: display ASCII QR inside terminal
		tip := lipgloss.NewStyle().Foreground(colorGold).Render("请打开【抖音 APP】或【番茄小说 APP】扫描下方二维码：")
		qrBox := lipgloss.NewStyle().
			Background(lipgloss.Color("#ffffff")).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Align(lipgloss.Center).
			Render(m.authTicket.QRArt)

		statusLine := lipgloss.NewStyle().Foreground(colorCyan).Render("状态: " + m.loginStatus)
		keys := lipgloss.JoinHorizontal(lipgloss.Center,
			keyCapKey.Render("o/Enter"), " ", keyCapDesc.Render("外部图片打开"),
			keyCapKey.Render("c"), " ", keyCapDesc.Render("粘贴Cookie"),
			keyCapKey.Render("s"), " ", keyCapDesc.Render("刷新二维码"),
			keyCapKey.Render("Esc"), " ", keyCapDesc.Render("取消返回"),
		)

		body := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s", header, tip, qrBox, statusLine, keys)
		return cardActiveBoxStyle.Width(min(76, m.width-4)).Render(body)
	}

	// Compact mode for small / laptop screens (height < 40 or width < 72)
	screenTip := lipgloss.NewStyle().Foreground(colorGold).Render(
		fmt.Sprintf("💻 当前终端高度较紧凑（%d 行），为避免乱码，已为你生成高清二维码图片！", m.height),
	)

	openBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#1e1e2e")).
		Background(colorCyan).
		Padding(0, 2).
		Render("👉 按 [ Enter ] 或 [ o ] 键：立即弹出二维码图片")

	var pathInfo string
	if m.authTicket != nil && m.authTicket.ImagePath != "" {
		pathInfo = lipgloss.NewStyle().Foreground(colorDarkGray).Render("图片位置: " + m.authTicket.ImagePath)
	}

	statusLine := lipgloss.NewStyle().Foreground(colorCyan).Render("实时状态: " + m.loginStatus)

	keys := lipgloss.JoinHorizontal(lipgloss.Center,
		keyCapKey.Render("Enter / o"), " ", keyCapDesc.Render("打开二维码图片"),
		keyCapKey.Render("c"), " ", keyCapDesc.Render("粘贴Cookie"),
		keyCapKey.Render("s"), " ", keyCapDesc.Render("刷新"),
		keyCapKey.Render("Esc"), " ", keyCapDesc.Render("返回"),
	)

	body := fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s\n\n%s",
		header, screenTip, openBtn, pathInfo, statusLine, keys)
	return cardActiveBoxStyle.Width(min(72, m.width-4)).Render(body)
}

func (m Model) viewAddBook() string {
	header := headerTitleStyle.Render("🔗 导入番茄小说")
	inputView := m.textInput.View()
	keys := lipgloss.JoinHorizontal(lipgloss.Center,
		keyCapKey.Render("Enter"), " ", keyCapDesc.Render("确认提交"),
		keyCapKey.Render("Ctrl+V"), " ", keyCapDesc.Render("一键粘贴"),
		keyCapKey.Render("Esc"), " ", keyCapDesc.Render("取消返回"),
	)
	content := fmt.Sprintf("%s\n\n%s\n\n%s", header, inputView, keys)
	return cardActiveBoxStyle.Width(min(70, m.width-4)).Render(content)
}

func (m Model) viewToc() string {
	var b strings.Builder
	title := "章节目录"
	if m.currentBook != nil {
		title = fmt.Sprintf("《%s》- 目录", m.currentBook.Title)
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n")

	if m.tocSearching {
		b.WriteString("搜索: " + m.tocSearchInput.View() + "\n\n")
	} else {
		b.WriteString(dimStyle.Render("按 [/] 键可输入章节关键词或序号快速过滤\n\n"))
	}

	chList := m.filteredChapters
	if len(chList) == 0 {
		b.WriteString(dimStyle.Render("  未搜索到匹配章节\n"))
	} else {
		start := max(0, m.tocCursor-8)
		end := min(len(chList), start+18)

		for i := start; i < end; i++ {
			ch := chList[i]
			prefix := "  "
			style := itemNormalStyle
			if i == m.tocCursor {
				prefix = "▶ "
				style = itemSelectedStyle
			}

			lockMark := ""
			if ch.Locked {
				lockMark = " [VIP]"
			}

			readMark := ""
			if ch.Index == m.currChapterIdx {
				readMark = " (当前阅读)"
			}

			line := fmt.Sprintf("%s[%d] %s%s%s", prefix, ch.Index, ch.Title, lockMark, readMark)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	statusBar := statusBarStyle.Render(
		fmt.Sprintf("匹配 %d / %d 章  %s 选定章节  %s 过滤搜索  %s 返回",
			len(chList), len(m.chapters),
			statusKeyStyle.Render("[Enter]"),
			statusKeyStyle.Render("[/]"),
			statusKeyStyle.Render("[Esc]"),
		),
	)
	b.WriteString("\n" + statusBar)
	return b.String()
}

func (m Model) viewReader() string {
	if m.currChapter == nil {
		return "正文未加载"
	}

	// 1. Single-line mode: authentic mock screen on top, novel ticker on bottom
	if m.stealthMode == masquerade.ModeStatusLine {
		fakeTop := m.getFakeBackground()
		visibleText, pct := masquerade.ExtractMiniTicker(m.currChapter.Paragraphs, m.miniTickerOffset, m.width-30)

		miniLine := fmt.Sprintf("[%s 第%d章 %d%%] %s",
			m.currentBook.Title, m.currChapterIdx, pct, visibleText)

		helpLine := dimStyle.Render(" [Space/→]前进 [←]后退 [m]切模式 [c]切背景 [b/Esc]老板键")
		return fakeTop + "\n" + highlightStyle.Render(miniLine) + helpLine
	}

	// 2. Standard Viewport Header
	var b strings.Builder
	modeTag := fmt.Sprintf("[%s]", m.stealthMode.Name())
	scrollTag := ""
	if m.autoScroll {
		scrollTag = " [自动滚屏]"
	}

	header := fmt.Sprintf(" 《%s》- 第 %d 章: %s %s%s",
		m.currentBook.Title, m.currChapterIdx, m.currChapter.Title, modeTag, scrollTag)
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")

	// 3. Viewport Content
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// 4. Status Bar
	statusBar := statusBarStyle.Render(
		fmt.Sprintf("%s 滚动  %s 换模式  %s 自动滚屏  %s 上/下章  %s 目录  %s 离线后20章  %s 老板键  %s 书架",
			statusKeyStyle.Render("[j/k/d/u]"),
			statusKeyStyle.Render("[m]"),
			statusKeyStyle.Render("[a]"),
			statusKeyStyle.Render("[p/n]"),
			statusKeyStyle.Render("[t]"),
			statusKeyStyle.Render("[D]"),
			statusKeyStyle.Render("[b/Esc]"),
			statusKeyStyle.Render("[q]"),
		),
	)
	b.WriteString(statusBar)
	return b.String()
}

func (m Model) viewBoss() string {
	return m.getFakeBackground()
}

func (m Model) viewLoading() string {
	return fmt.Sprintf("\n  %s %s\n", m.spinner.View(), m.statusMsg)
}
