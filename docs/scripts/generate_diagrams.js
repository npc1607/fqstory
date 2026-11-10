const { DiagramBuilder } = require('/Users/pcong/.pi/agent/skills/excalidraw-diagram/scripts/diagram_generator.js');

// 1. TUI 交互与摸鱼模式状态流转图
new DiagramBuilder({
  preset: "blueprint",
  title: "fqstory - 番茄小说 Go TUI 摸鱼阅读器 交互与状态流转",
  subtitle: "基于 Charm Bubbletea 的状态流转、摸鱼伪装矩阵与老板键应急机制",
  width: 1200,
  height: 720,
  grid: true,
  groups: [
    { label: "1. 导航与书库管理", x: 40, y: 110, w: 320, h: 560, theme: "blue" },
    { label: "2. 核心阅读器 (Reader Engine)", x: 390, y: 110, w: 420, h: 560, theme: "teal" },
    { label: "3. 摸鱼特化伪装层 (Stealth Masquerade)", x: 840, y: 110, w: 320, h: 560, theme: "indigo" }
  ],
  nodes: [
    // 导航区
    { id: "shelf", shape: "card", title: "书架视图 (ShelfView)", sub: "默认启动入口", x: 70, y: 160, w: 260,
      items: ["展示历史书架 / 最近阅读", "实时章节更新红点", "[Enter] 进入上次阅读", "[/] 切换搜索模式", "[d] 移出书架 / [r] 刷新"], theme: "blue" },
    { id: "search", shape: "card", title: "全网搜索 (SearchView)", sub: "番茄 API 聚合检索", x: 70, y: 340, w: 260,
      items: ["输入书名 / 作者关键词", "展示排行榜与热门推荐", "[Enter] 展开书籍详情/目录", "[a] 加入书架"], theme: "slate" },
    { id: "toc", shape: "card", title: "目录导引 (TocView)", sub: "章节树与倒序支持", x: 70, y: 500, w: 260,
      items: ["快速定位目标章节", "[j/k] 上下翻动章节", "[/] 章节内标题过滤", "[g/G] 顶部/底部直达"], theme: "slate" },

    // 阅读器核心
    { id: "reader", shape: "card", title: "多模态阅读器 (ReaderView)", sub: "Bubbletea Viewport", x: 420, y: 160, w: 360,
      items: ["自适应终端宽度折行与排版", "平滑滚动 [j/k/d/u]", "后台无感预加载后 5 章", "实时记录段落级阅读偏移量", "[m] 循环切换摸鱼伪装模式", "[b / Esc] 触发老板键"], theme: "teal" },
    
    { id: "mode_classic", shape: "rect", title: "A. 经典阅读 (Classic)", sub: "极简暗色终端，低对比度", x: 440, y: 330, w: 320, h: 64, theme: "zinc" },
    { id: "mode_log", shape: "rect", title: "B. 日志伪装 (Log Stream) ★", sub: "格式化为 [INFO] 后端日志，自动滚屏", x: 440, y: 410, w: 320, h: 64, theme: "green" },
    { id: "mode_code", shape: "rect", title: "C. 代码注释 (Code Comment)", sub: "伪装为 Go/Rust 代码注释段落", x: 440, y: 490, w: 320, h: 64, theme: "cyan" },
    { id: "mode_mini", shape: "rect", title: "D. 极简单行 (Status Line)", sub: "仅占底栏 1 行，其余为假终端", x: 440, y: 570, w: 320, h: 64, theme: "yellow" },

    // 摸鱼层
    { id: "boss_key", shape: "diamond", title: "老板键 (Boss Key)", sub: "按 [b] / [Esc] 触发", x: 890, y: 160, w: 220, h: 100, theme: "red" },
    { id: "fake_screen", shape: "card", title: "假终端伪装屏幕", sub: "BossOverlay", x: 870, y: 310, w: 260,
      items: ["选项 1: go test -v 流式输出", "选项 2: htop 动态伪监控", "选项 3: git diff 假代码对比", "选项 4: 挂起切出真实终端", "按任意配置热键静默返回"], theme: "dark" },
    { id: "storage", shape: "cylinder", title: "本地离线库 (SQLite)", sub: "离线章节 / 进度 / 密钥", x: 870, y: 510, w: 260, h: 110, theme: "indigo" }
  ],
  edges: [
    { from: "shelf", to: "search", label: "按 [/] 搜索", routing: "orthogonal" },
    { from: "search", to: "toc", label: "查看目录", routing: "orthogonal" },
    { from: "toc", to: "reader", label: "选中章节", routing: "orthogonal" },
    { from: "shelf", to: "reader", label: "继续阅读", routing: "orthogonal" },
    { from: "reader", to: "mode_classic", label: "模式切换 [m]", routing: "straight" },
    { from: "mode_classic", to: "mode_log", routing: "straight" },
    { from: "mode_log", to: "mode_code", routing: "straight" },
    { from: "mode_code", to: "mode_mini", routing: "straight" },
    { from: "reader", to: "boss_key", label: "[b] 紧急避险", routing: "orthogonal" },
    { from: "boss_key", to: "fake_screen", label: "瞬切假界面", routing: "orthogonal" },
    { from: "fake_screen", to: "reader", label: "再次唤醒", strokeStyle: "dashed", routing: "orthogonal" },
    { from: "reader", to: "storage", label: "实时进度自动落盘", strokeStyle: "dashed", routing: "orthogonal" }
  ]
}).generate('./docs', 'fqstory-interaction');

// 2. 系统技术架构与分层设计图
new DiagramBuilder({
  preset: "pastel",
  title: "fqstory - 系统分层架构与数据流向",
  subtitle: "Go 终端应用架构：UI表现层、摸鱼引擎、业务领域层、离线存储与爬虫协议层",
  width: 1200,
  height: 720,
  grid: true,
  groups: [
    { label: "1. TUI 表现层 (Presentation Layer - Bubbletea & Lipgloss)", x: 40, y: 110, w: 1120, h: 140, theme: "indigo" },
    { label: "2. 摸鱼伪装与渲染管道 (Stealth Masquerade Pipeline)", x: 40, y: 270, w: 1120, h: 130, theme: "teal" },
    { label: "3. 业务调度与状态引擎 (Domain & State Management)", x: 40, y: 420, w: 1120, h: 130, theme: "blue" },
    { label: "4. 基础设施与数据提供层 (Infrastructure & Provider)", x: 40, y: 570, w: 1120, h: 120, theme: "slate" }
  ],
  nodes: [
    // 表现层
    { id: "ui_shelf", shape: "rect", title: "Shelf Model", sub: "书架列表 / 进度标识", x: 70, y: 150, w: 240, h: 70, theme: "indigo" },
    { id: "ui_search", shape: "rect", title: "Search Model", sub: "模糊搜索 / 榜单浏览", x: 340, y: 150, w: 240, h: 70, theme: "indigo" },
    { id: "ui_reader", shape: "rect", title: "Reader Viewport", sub: "滚动排版 / 翻页控制", x: 610, y: 150, w: 240, h: 70, theme: "indigo" },
    { id: "ui_overlay", shape: "rect", title: "Boss Key Overlay", sub: "伪装终端全屏接管", x: 880, y: 150, w: 240, h: 70, theme: "indigo" },

    // 摸鱼引擎
    { id: "masq_log", shape: "rect", title: "LogFormatter", sub: "微服务日志包装器", x: 120, y: 305, w: 270, h: 65, theme: "teal" },
    { id: "masq_code", shape: "rect", title: "CodeCommenter", sub: "代码注释与AST模拟", x: 460, y: 305, w: 270, h: 65, theme: "teal" },
    { id: "masq_sim", shape: "rect", title: "SysOutputSimulator", sub: "假测试/编译/htop输出", x: 800, y: 305, w: 270, h: 65, theme: "teal" },

    // 领域与状态管理
    { id: "state_mgr", shape: "rect", title: "BookStateManager", sub: "当前章节/段落偏移量", x: 120, y: 455, w: 270, h: 65, theme: "blue" },
    { id: "prefetch_mgr", shape: "rect", title: "BackgroundPrefetcher", sub: "智能预读队列 (后5章)", x: 460, y: 455, w: 270, h: 65, theme: "blue" },
    { id: "decrypt_engine", shape: "rect", title: "FontDecryptor", sub: "PUA 映射/字库解码", x: 800, y: 455, w: 270, h: 65, theme: "blue" },

    // 基础设施
    { id: "storage_sqlite", shape: "cylinder", title: "SQLite / 本地数据库", sub: "书架 / 历史 / 离线缓存", x: 140, y: 590, w: 240, h: 80, theme: "slate" },
    { id: "api_fanqie", shape: "rect", title: "Fanqie API Client", sub: "APP 网关 / Web 搜索", x: 480, y: 600, w: 240, h: 65, theme: "slate" },
    { id: "local_importer", shape: "rect", title: "Local TXT/EPUB Driver", sub: "离线无网导入兜底", x: 810, y: 600, w: 240, h: 65, theme: "slate" }
  ],
  edges: [
    { from: "ui_reader", to: "masq_log", routing: "orthogonal" },
    { from: "ui_overlay", to: "masq_sim", routing: "orthogonal" },
    { from: "masq_log", to: "state_mgr", routing: "orthogonal" },
    { from: "state_mgr", to: "prefetch_mgr", routing: "straight" },
    { from: "prefetch_mgr", to: "storage_sqlite", routing: "orthogonal" },
    { from: "prefetch_mgr", to: "api_fanqie", routing: "orthogonal" },
    { from: "api_fanqie", to: "decrypt_engine", routing: "straight" }
  ]
}).generate('./docs', 'fqstory-architecture');
