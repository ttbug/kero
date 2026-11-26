# Kero 进度

## 当前阶段

- 已处理 `main`（`6de2a2d`）合入 `feat/one` 的图标与历史会话标题冲突，同时保留 Pi 图标／transcript 标题、Antigravity 彩色图标及当前分支的标题优先级和恢复行为；专项回归、race、静态检查及深浅色渲染通过，全量测试仍有已记录的五项失败。
- Xview 应用名称及品牌图标替换已实现并专项验证：采用用户第二张图的彩色图案，修复系统为原透明位图补白底的问题，主图现为深色铺满的实心正方形；菜单栏使用简化的单色 X。本地 `Xview.app` 构建成功，macOS 原生图标渲染与静态检查通过，全量测试仍有此前已记录的五项失败。
- 左侧边栏已完成第一行真实会话标题升级（保留第二行 `状态 · Agent/程序名` 不变；Agent 会话优先级为 `手动命名` > `Claude Code transcript 标题` > `Agent 首条有效用户输入` > `新会话` / `New Session`，不再采用终端 OSC 标题以免 `codex|...` 等状态串覆盖会话标题，同步更新提示卡与双击重命名逻辑）；专项回归、race 检查、渲染截图检查、`go tool mygo vet .` 与 `go tool mygo i18n check .` 均通过，全量测试仅存在既有失败项。
- 左侧边栏已完成卡片与树形动态数据流样式改造（含品牌弧线装饰、项目计数徽章、项目卡片及 `Files & Diff`/`Project Files` 子行、从分割线垂直引出的圆角中性树干及仅在 `Working` 会话触发的动态数据流脉冲、会话图标容器与状态/程序副标题、左下角 Agent 运行计数与待审批提示卡片），并新增对 Antigravity（`agy` / `antigravity` / `antigravity-cli`）的进程识别、状态探测、官方彩色图标与侧边栏展示支持；专项回归、多主题渲染检查、`go tool mygo vet .` 与 `go tool mygo i18n check .` 均通过，全量测试存在既有/未修改包失败项。
- 左侧栏参考布局及主题联动修复已完成，专项回归和渲染检查通过；全量测试存在未通过项。
- 已处理 `main`（`2c7c487`）合入 `feat/own` 的冲突及恢复流程死锁，相关回归通过；尚未创建合并提交。
- 已处理本轮 `main`（`4d09d61`）合入的项目改名冲突，相关回归与 race 检查通过；尚未创建本轮合并提交。
- 已处理 `main`（`e2b5bd1`）合入 `feat/own` 的目录探测冲突，专项回归与直接相关的 race 检查通过；尚未创建本轮合并提交。
- 本文件从本次任务开始记录，其他项目进度待确认。

## 已完成

- `6de2a2d` 合并冲突：`icons.go` 同时注册 Pi 与 Antigravity，保留彩色位图渲染；`paneConversation` 优先采用有效 `AgentTitle`，统一使用既有 Agent 状态串过滤及 Pi 的 `π - 目录名` 过滤，保留恢复时回填标题逻辑。新增表驱动回归覆盖优先级、Pi 默认标题、Codex 状态串、路径及 Agent 名称。
- Xview 应用名称和图标：
  - `mygo.json` 的打包名称与 macOS 权限说明改为 `Xview`，同步默认窗口标题、无会话标题、非 macOS 应用菜单名称、设置页版本名称和测试通知标题；内部命令、应用标识、数据目录及远程协议保持原有约定。
  - `resources/icon.png` 与 `resources/icon-dev.png` 采用用户第二张图中的彩色 X 和终端图案，经内置 imagegen 修正底板后导出为 1024×1024、所有像素 alpha 为 255 的深色实心 PNG，移除透明留白及预绘圆角边界，由系统裁出圆角；`icons/tray.png` 为此前导出的 64×64 彩色托盘图标。两种构建暂时共用同一主图稿。
  - `icons/tray-template.png` 为从原图提取并简化的三段式单色 X，去掉底板、窗口、网格及发光效果，36×36 透明 PNG；大图源稿保存在 `resources/xview-menubar.png`，处理方式记录于 `icons/README.md`。
  - 已检查透明角点、深浅背景上的图标和 18／36 像素单色图标预览，以及深浅侧栏渲染；本地 darwin/arm64 临时签名构建成功，核对 `Xview.app` 的名称、可执行文件名及 `AppIcon.icns` 资源，未启动桌面应用验证真实 Dock／菜单栏显示。
  - 后续通过 macOS 27.0.1 的 `NSWorkspace.shared.icon(forFile:)` 原生渲染复现白色底板：原 PNG／ICNS 本身无白框，但系统将其缩小并补上白色底板；改用铺满画布的深色实心主图后，重新构建并经同一原生接口确认白底消失，修复前后预览为 `/tmp/xview-native-icon-before.png`、`/tmp/xview-native-icon-after.png`。未替换已安装应用或修改系统图标缓存。
- 侧边栏会话第一行标题升级（第二行保持 `状态 · Agent/程序名` 不变）：
  - `internal/proto/proto.go` 新增 `Pane.AgentTitle`、`IsGenericAgentTitle`（同步过滤 `codex|kero|Ready|...` 类结构化状态标题）、`Pane.AgentConversationTitle`、`Pane.ConversationTitle` 与 `Session.SidebarTitle`（Agent 会话忽略终端 OSC 标题，严格遵循 `Name` > `AgentTitle` > `新会话`），并在 `DisplayName` 优先返回 `ConversationTitle()`；`internal/proto/layout_test.go` 与 `internal/proto/proto_test.go` 新增 `TestIsGenericAgentTitle` 与 `TestSidebarTitle`。
  - `internal/daemon/prompt_title.go` 与 `internal/daemon/session.go` / `rpc.go` / `conversations.go` / `store.go` 实现基于 rune+光标的 Agent 首条有效用户输入捕获（支持 `Home`/`End`、`Left`/`Right`、`Delete`、`Ctrl+A/E/B/F/D/K/U/W`、UTF-8 退格/跨包分片、`ESC`、Bracketed Paste、Kitty/修饰键换行，屏蔽审批 `Waiting` 按键与 `/model`/`y`/`1`/路径等噪声，遇到方向上下键/`Tab`/修饰词跳等不可靠操作安全放弃推断，支持 `/clear`/`/new`/`/reset` 与退出/恢复失败/切换 Agent 清零）、活跃 Claude Code transcript 标题自动同步，以及历史记录优先级统一与 `resumeConversation` 标题回填和状态持久化（含 `AgentTitleKind`）；新增 `TestPromptTitleTracker`、`TestPaneAgentTitleTracking`、`TestTickClearsStaleRestoredAgentTitle`、`TestRestoreAgentTitleKind` 与 `TestConversationTitleIgnoresCodexStatusAndRestoresTitle`。
  - `locales/en.json`、`locales/zh-Hans.json`、`locales/ja.json` 新增 `sidebarNewSession`（`New Session` / `新会话` / `新しいセッション`）并由 `go tool mygo generate` 重新生成 `locales/catalog_gen.go`。
  - `view.go`（`sessionTitle`、会话行第一行样式微调、`footerSessionTip`、`sidebarAttentionCard`）与 `app.go`（`startRename` 记录 `renameInit`、`commitRename` 消除竞态）接入 `SidebarTitle`；`sidebar_test.go` 新增 `TestSidebarSessionTitle`。
- 左侧边栏卡片树形重构（按用户确认省略顶部 Dashboard/Matrix、暂不新增 Pin 置顶、副标题状态后显示 Agent/程序名）：
  - 在 `theme.go` 新增 `sideCard`、`sideCardBorder`、`sideIconBg`、`sideTree` 调色板字段，并按深浅色与 Ghostty 主题动态派生。
  - 在 `view.go` 顶部品牌区下方添加弧线光点装饰；`sidebarProjectSection` 拆分项目数字徽章与右侧会话数；`groupBody` 为项目增加独立圆角边框卡片、常驻 `+` 新建会话按钮及 `Files & Diff` / `Project Files` 入口行（`openProjectSide` 支持切换项目会话并打开右侧面板）；`sessionRow` / `sessionTree` / `treeElbowPath` / `treeDataFlow` / `workingDots` 实现从分割线引出的圆角中性树形连线，并仅在存在 `Working` 会话时沿连续路径向下绘制渐隐拖尾的动态数据脉冲；`sidebarFooter` 与 `sidebarAttentionCard` 在左下角展示运行中/待审批 Agent 计数、悬停会话详情并在存在待审批会话时于底部列出可点击直达（自动展开折叠项目）的审批清单。
  - 在 `project_order.go` 调整项目拖拽插入线偏移以匹配卡片间距；在 `sidebar_test.go` 与 `view_test.go` 更新布局、主题与悬浮提示回归并新增 `TestSidebarProjectFilesRow`。
- 新增 Antigravity Agent 识别与侧边栏支持：
  - `internal/daemon/daemon.go` 加入 `"antigravity"` 及 `agy` / `antigravity-cli` 别名与脚本路径匹配；`internal/proto/proto.go` 在 `AgentNames` 注册 `"antigravity": "Antigravity"`。
  - `internal/detect/manifests/antigravity.toml` 与 `internal/detect/detect.go`（`agy` 映射）新增运行中、审批/问答阻塞与空闲态规则；`icons/antigravity.png`（官方彩色徽标）、`icons/antigravity.svg`（同步官方轮廓）与 `icons.go`（`programBitmap` / `renderProgramIcon`）接入彩色位图图标并在 `view.go`、`ended.go`、`panes.go`、`palette.go` 统一使用（退出/历史态降低不透明度）。
  - `internal/daemon/session_test.go`、`internal/detect/detect_test.go` 与 `sidebar_test.go` 新增 `TestDetectAgentAntigravity`、`TestAntigravity` 与 `TestSidebarAntigravity`（含位图解码校验）。
- 补齐 `AGENTS.md`，明确目录、MyGo 生命周期和验证约定。
- 项目标题显示项目数与会话数，折叠箭头移至左侧，项目名和分支标签使用当前主题调色板。
- 会话显示程序名、真实会话名称与已有创建时间，并绘制两层树形连线。
- 选中会话保留圆角背景和左侧强调线，文字、选中态及强调色与右侧共用主题调色板。
- 移除侧栏固定配色和独立底色，恢复窗口主题背景，保留树形布局。
- 「最近任务」复用已结束的历史会话；启用历史记录且为空时显示零，关闭历史记录时只显示标题。
- 已检查深色、浅色、窄侧栏长名称及展开多窗格的测试渲染。
- 新增实际渲染回归，覆盖同一窗口内切换 Paper 深浅色、TokyoNight Moon、Catppuccin Mocha、TokyoNight Day、Plain 深浅色，检查背景和选中强调线跟随主题。
- 合并保留会话启动失败后的 worktree 清理与空自动项目清理，回归覆盖手动项目保留、自动项目移除和同一分支重试。
- 修复自动合并后的恢复死锁：`restore()` 持锁期间调用 `pruneAutoProjectsLocked()`，避免重复获取 daemon 锁；恢复专项与并发 race 检查通过。
- 本轮项目改名冲突同时保留 `Auto = false` 与 `found = true`：用户改名的自动项目在清空后保留，已存在项目改名成功，未知项目返回错误；现有回归覆盖并通过。
- 目录探测合并保留分支每 400ms 跟随 shell 目录的行为，同时使用 `main` 的存活进程组成员读取程序目录；补充回归覆盖管道首进程退出、存活成员切换目录的情况。

## 进行中

- 暂无。

## 待办

- 真实桌面窗口的最终视觉确认待完成，本次使用 MyGo Tester 渲染检查。
- 单独排查全量测试失败，超出本次侧边栏样式范围。

## 阻塞

- 全量测试未通过，不能宣称全仓验证通过。
- `TestRightSidebarMakesRoom`、`TestSplitHost`、`TestBridgeScript`、`TestImageStorageLimitBounds/NaN` 在临时目录的未修改 `HEAD` 副本上同样失败。
- `TestCommandInForeground/zsh` 和 `TestCommandTellsExit/zsh` 在当前目录的全量、隔离复测均失败；临时基线副本通过，原因待确认。
- 本轮主题修复后的全量测试仍有五项失败：右侧栏收窄、两项 Zsh、远程桥接和图片存储限制；`TestSplitHost` 本轮通过。
- `e2b5bd1` 合并验证扩大 race 范围时，`TestRestoreKeepsTheZoom` 启动的恢复流程触发 `restorePane()` 中无锁写 `p.dirty` 与 `pane.output()` 写同一字段的竞态；两侧合并前代码均有该逻辑，本轮未修改，待单独修复。

## 最近验证

- 2026-11-26（`6de2a2d` 合并冲突）：`go tool mygo vet .`、`git diff --check` 通过；会话专项与 race 检查通过（`TestPaneConversationMergedTitles`、`TestPiTitle`、`TestClaudeTitle`、`TestConversationTitleIgnoresCodexStatusAndRestoresTitle`、`Test.*Conversation`，`2.484s`）；图标与侧栏专项通过（`TestProgramIconPi`、`TestSidebarAntigravity`、`TestSidebarSessionTitle`，`1.375s`），已检查 Antigravity 深浅色渲染。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败仍为已记录的 `TestRightSidebarMakesRoom`、`TestNewSaysAGoneWorkingDirectory`、`TestCommandInForeground/zsh`、`TestCommandTellsExit/zsh`、`TestBridgeScript`；日志为 `/tmp/kero-merge-6de2a2d-full.log`，截图为 `/tmp/kero-merge-6de2a2d-shots/`。未进行真实桌面窗口验证。
- 2026-11-26（Xview 应用图标白底修复）：已先用 macOS 原生图标接口复现系统合成白色底板，再修正主图底板和透明通道；源图检查确认 1024×1024、alpha 全为 255，原生渲染后四个边缘检查点由亮色变为深色，白色底板消失。`go tool mygo vet .`、`git diff --check` 通过；本地临时签名构建通过，产物为 `build/xview-icon-no-border/darwin-arm64/Xview.app`，未安装或发布。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束，仍为此前五项失败（右侧栏、失效工作目录、两项 Zsh、远程桥接），日志为 `/tmp/xview-icon-no-border-tests.log`。本轮未改动 Go 逻辑或菜单栏图标。
- 2026-11-26（Xview 名称与图标替换）：`go tool mygo vet .`、`git diff --check`、PNG 尺寸及透明通道检查通过；`CGO_ENABLED=0 TMPDIR=/tmp KERO_SHOTS=/tmp/xview-brand-shots go test -v -run 'TestSidebarBrandAndTasks|TestSidebarReferenceLayout|TestAwakeTray|Test.*Menu|Test.*Settings' .` 通过（`4.512s`），已检查图标深浅背景预览 `/tmp/xview-brand-preview.png` 和侧栏渲染。`CGO_ENABLED=0 TMPDIR=/tmp go tool mygo build -platform darwin/arm64 -o build/xview-brand-check -sign - -skip-notarize -skip-dmg .` 通过，产物为 `build/xview-brand-check/darwin-arm64/Xview.app`，未公证、未安装、未发布。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项与本文件此前记录一致：`TestRightSidebarMakesRoom`、`TestNewSaysAGoneWorkingDirectory`、`TestCommandInForeground/zsh`、`TestCommandTellsExit/zsh`、`TestBridgeScript`；日志为 `/tmp/xview-brand-full.log`。
- 2026-11-26（3 项 P2 标题与历史记录问题修复）：统一 `proto.IsGenericAgentTitle` 过滤 `codex|kero|Ready|...` 状态串并调整 `paneConversation` 优先读取 `AgentTitle`、`resumeConversation` 回填 `AgentTitle` 与 `agentTitleKind`、以及 `promptTitleTracker` 改为基于 rune+光标支持常见光标编辑并在方向上下键/`Tab`/修饰词跳等不可靠场景安全放弃推断；新增 `TestConversationTitleIgnoresCodexStatusAndRestoresTitle` 与光标/异常操作单测。`gofmt -l`（本次改动文件）、`git diff --check`、`go tool mygo vet .` 通过；`CGO_ENABLED=0 TMPDIR=/tmp go test -race ./internal/proto ./internal/daemon -run 'Test.*Title|TestDisplayName|TestConsoleTitle|TestClaude|TestResumeConversation|TestConversation|TestDetectAgent|TestRestore'` 通过（`proto` `2.052s`、`daemon` `5.885s`）；`CGO_ENABLED=0 TMPDIR=/tmp go test -v -run 'TestSidebar|TestSessionHoverClose|TestSessionStatusLabel|TestFold|TestNeighbor|TestGroupBranch' .` 全部通过（`8.429s`）。
- 2026-11-26（侧边栏 Agent 会话标题移除 OSC 标题依赖）：Agent 会话标题严格按 `手动改名` > `Agent 会话标题 (AgentTitle)` > `首条有效用户输入 (AgentTitle)` > `新会话`，不再读取终端 OSC 标题（避免 `codex|kero|Ready|...` 覆盖标题）；`gofmt -l`、`git diff --check`、`go tool mygo vet .` 通过；`CGO_ENABLED=0 TMPDIR=/tmp go test -v ./internal/proto`（`1.175s`）与 `CGO_ENABLED=0 TMPDIR=/tmp KERO_SHOTS=/tmp/kero-sidebar-title-shots go test -v -run 'TestSidebar|TestSessionHoverClose|TestSessionStatusLabel|TestFold|TestNeighbor|TestGroupBranch' .`（`9.629s`）全部通过，并检查更新后的 `sidebar-titles-dark.png`。
- 2026-11-26（侧边栏会话标题 review 问题修复）：完成 `internal/daemon/store.go` `gofmt` 对齐、持久化与恢复 `AgentTitleKind` 及恢复失败/跨 Agent 残留清零（[session.go](file:///Users/JOYY/workspace/kero/internal/daemon/session.go)、[store.go](file:///Users/JOYY/workspace/kero/internal/daemon/store.go)）与 `commitRename` 竞态修复（[app.go](file:///Users/JOYY/workspace/kero/app.go)），新增 `TestRestoreAgentTitleKind` 与 `TestTickClearsStaleRestoredAgentTitle`；本次修改的 Go 文件 `gofmt -l` 无输出，`git diff --check` 与 `go tool mygo vet .` 通过；`CGO_ENABLED=0 TMPDIR=/tmp go test -race ./internal/proto ./internal/daemon -run 'Test.*Title|TestDisplayName|TestConsoleTitle|TestClaude|TestResumeConversation|TestDetectAgent|TestRestore'` 通过（`proto` 命中缓存、`daemon` `5.640s`）；`CGO_ENABLED=0 TMPDIR=/tmp go test -v -run 'TestSidebar|TestSessionHoverClose|TestSessionStatusLabel|TestFold|TestNeighbor|TestGroupBranch' .` 全部通过（`8.464s`）；`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项仍为上述五项既有/未修改包测试（日志：`/tmp/kero-sidebar-titles-fix-full.log`）。
- 2026-11-26（侧边栏第一行会话标题升级）：`go tool mygo generate`、`go tool mygo i18n check .`、`go tool mygo vet .`、`git diff --check` 通过；`CGO_ENABLED=0 TMPDIR=/tmp go test -race ./internal/proto ./internal/daemon -run 'Test.*Title|TestDisplayName|TestConsoleTitle|TestClaude|TestResumeConversation|TestDetectAgent'` 通过（`proto` `2.190s`、`daemon` `1.873s`）；`CGO_ENABLED=0 TMPDIR=/tmp KERO_SHOTS=/tmp/kero-sidebar-title-shots go test -v -run 'TestSidebar|TestSessionHoverClose|TestSessionStatusLabel|TestFold|TestNeighbor|TestGroupBranch' .` 全部通过（`9.888s`），并检查 `sidebar-titles-dark.png`、`sidebar-titles-light.png` 与 `sidebar-reference-dark.png`。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项仍为五项既有/未修改包测试（`TestRightSidebarMakesRoom`、`TestNewSaysAGoneWorkingDirectory`、`TestCommandInForeground`、`TestCommandTellsExit`、`TestBridgeScript`，日志：`/tmp/kero-sidebar-titles-full.log`）；未完成真实桌面窗口人工目视确认。
- 2026-11-26（接入 Antigravity 官方彩色图标）：`go tool mygo vet .`、`go tool mygo i18n check .`、`git diff --check` 通过；`TestSidebarAntigravity`（含 `programBitmap` 校验）、`TestSidebarReferenceLayout`、`TestSidebarFooterAgentStatus`、`TestSidebarWorkingDataFlow`、`TestSidebarFollowsTheme`、`TestSidebarSplitPanes` 与 `TestPalette*` 全部通过（`5.943s`），并检查 `sidebar-antigravity-dark.png` 与 `sidebar-antigravity-light.png`。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项仍为上述五项既有/未修改包测试（日志：`/tmp/kero-antigravity-color-full.log`）。
- 2026-11-26（Antigravity Agent 识别与侧边栏展示）：`go tool mygo vet .`、`go tool mygo i18n check .`、`git diff --check` 通过；`TestSidebarAntigravity`、`TestSidebarReferenceLayout`、`TestSidebarFooterAgentStatus`、`TestSidebarWorkingDataFlow`、`TestSidebarFollowsTheme`、`./internal/detect`（含 `TestAntigravity` 与 `TestManifestsStrict`）、`./internal/proto` 及 `TestDetectAgentAntigravity`（`./internal/daemon`）全部通过，并检查 `sidebar-antigravity-dark.png` 与 `sidebar-antigravity-light.png`。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项仍为上述五项既有/未修改包测试（日志：`/tmp/kero-antigravity-full.log`）。
- 2026-11-26（左下角 Agent 运行状态与待审批卡片）：`go tool mygo vet .`、`go tool mygo i18n check .`、`git diff --check` 通过；新增 `TestSidebarFooterAgentStatus`（验证 `N 运行中`、`N 需要你`、折叠项目内待审批会话点击直达与窄侧栏不溢出），专项回归与右侧栏测试全部通过（`11.170s`），并检查 `sidebar-footer-agents-dark.png`、`sidebar-footer-agents-light.png`、`sidebar-reference-dark.png`、`sidebar-split-panes-dark.png`。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项仍为上述五项既有/未修改包测试（日志：`/tmp/kero-sidebar-footer-full.log`）。
- 2026-11-26（运行中树形动态数据流改造）：`go tool mygo vet .`、`go tool mygo i18n check .`、`git diff --check` 通过；更新 `TestSidebarFollowsTheme`（校验 7 主题品牌弧线中心强调色）并新增 `TestSidebarWorkingDataFlow`（验证空闲选中态无动画、运行态树干像素随时间流动并导出 `sidebar-dataflow-dark.png`），专项回归与右侧栏测试全部通过（`9.893s`），并检查 `sidebar-reference-dark.png`、`sidebar-reference-light.png`、`sidebar-dataflow-dark.png`、`sidebar-theme-tokyonight-moon.png`。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项仍为上述五项既有/未修改包测试（日志：`/tmp/kero-sidebar-dataflow-full.log`）。
- 2026-11-26（连接线圆角发光样式优化）：`go tool mygo vet .`、`go tool mygo i18n check .`、`git diff --check` 通过；扩展 `TestSidebarReferenceLayout` 覆盖单会话 L 形与多会话贯通圆角树形分支，专项回归与右侧栏测试全部通过（`9.277s`），并检查 `sidebar-reference-dark.png`、`sidebar-reference-light.png`、`sidebar-split-panes-dark.png`、`sidebar-theme-tokyonight-moon.png`。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过，失败项仍为上述五项既有/未修改包测试（日志：`/tmp/kero-sidebar-connectors-full.log`）。
- 2026-11-26（左侧边栏卡片树形改造）：`go tool mygo vet .`、`go tool mygo i18n check .`、`git diff --check` 通过；专项回归与右侧栏测试（`TestRightSidebar`、`TestCloseInViewer`、`TestSidebar*`、`TestFold*`、`TestNeighbor`、`TestGroupBranch`、`TestSessionStatusLabel`、`TestSessionHoverClose`、`TestSessionTooltipAtPointer`、`TestDoubleClickSidebarNewSession`、`TestDragProject*`、`TestProjectMenuMoves`、`TestReorderProjects`、`TestInsertionIndex`、`TestMoveProjectFromMenu`）全部通过（`8.031s`），并检查 `/tmp/kero-sidebar-redesign-shots/` 下深浅色、窄侧栏、分窗格及 7 种主题渲染截图。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束但未全通过：失败项为 `TestRightSidebarMakesRoom`、`TestNewSaysAGoneWorkingDirectory`（`internal/cli`）、`TestCommandInForeground` / `TestCommandTellsExit`（`internal/daemon`）、`TestBridgeScript`（`internal/remote`），日志：`/tmp/kero-sidebar-redesign-full.log`；未完成真实桌面窗口人工目视确认（本次通过 MyGo Tester 渲染验证）。
- 2026-11-22：`e2b5bd1` 合并的管道存活成员名称与目录、目录跟随、空项目保留、恢复、启动失败 worktree 清理和保存串行化专项通过；冲突直接相关的 race 检查通过，扩大范围发现上述恢复竞态。`go tool mygo vet .`、冲突文件 Go 格式与 Git diff 检查通过。`CGO_ENABLED=0 TMPDIR=/tmp go test ./... -timeout=120s` 正常结束，仍有 `TestRightSidebarMakesRoom`、两项 Zsh、`TestBridgeScript` 失败；日志：`/tmp/kero-merge-e2b5bd1-test.log`，专项 race 日志：`/tmp/kero-merge-e2b5bd1-race.log`。本轮手动修改限于 daemon 冲突、相关回归和本文件，未新增桌面渲染检查。
- 2026-11-22：项目、空项目保留、恢复及启动失败 worktree 清理专项通过；项目未知 ID、空项目和恢复流程的 race 检查通过；`go tool mygo vet .`、冲突文件 Go 格式及 Git diff 检查通过。
- 2026-11-22：`CGO_ENABLED=0 go test ./... -timeout=120s` 正常结束但未全通过：`TestRightSidebarMakesRoom`、两项 Zsh、`TestBridgeScript` 仍失败；新增 `TestDaemonStatusSaysAFailedState` 在默认临时目录下 Unix socket 绑定失败，使用 `TMPDIR=/tmp` 隔离复测通过。图片存储限制本轮未报失败。日志：`/tmp/kero-merge-20261122-test.log`。本轮只手动解决 daemon 冲突，未新增桌面视觉检查。
- 2026-11-21：合并冲突相关 worktree、空项目、浏览器恢复及会话恢复专项通过；`TestRestoreWhileState` 和 `TestWorktreeGoneWhenPaneFails` 的 race 检查通过。
- 2026-11-21：侧栏及七种主题渲染回归、`go tool mygo vet .`、`go tool mygo i18n check .`、Go 格式及 Git diff 检查通过。
- 2026-11-21：首轮全量在 `TestBrowserPane` 恢复阶段挂起，堆栈确认重复加锁；修复后 `CGO_ENABLED=0 go test ./... -timeout=120s` 正常结束，仍有上述五项已记录失败，未宣称全仓通过。日志：`/tmp/kero-merge-full-test-final.log`；首轮堆栈：`/tmp/kero-merge-full-test.log`。
- 2026-11-19：已检查工作区，开始修改前无未提交变更。
- 2026-11-19：侧边栏、折叠、相邻会话、分支、状态、悬停关闭、双击和项目拖动专项回归通过。
- 2026-11-19：`go tool mygo vet .`、`go tool mygo i18n check .` 和 `git diff --check` 通过。
- 2026-11-19：`CGO_ENABLED=0 go test ./...` 已执行，失败项及对照结果见上方阻塞记录。
- 2026-11-19：新增主题联动回归在修复前复现固定底色、固定强调色问题，修复后七种主题／外观组合全部通过。
- 2026-11-19：主题、侧栏、悬停关闭、折叠、分支和项目拖动专项回归通过；`go tool mygo vet .` 与 `git diff --check` 通过。
- 本轮预览：`/tmp/kero-sidebar-theme-shots/sidebar-theme-tokyonight-moon.png`、`/tmp/kero-sidebar-theme-shots/sidebar-theme-tokyonight-day.png`、`/tmp/kero-sidebar-theme-shots/sidebar-theme-catppuccin-mocha.png`。
- 本轮全量日志：`/tmp/kero-sidebar-theme-full-test.log`；首轮对照日志：`/tmp/kero-sidebar-full-test.log`、`/tmp/kero-sidebar-baseline-test.log`。
