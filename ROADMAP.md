# Kero 进度

## 当前阶段

- 左侧栏参考布局及主题联动修复已完成，专项回归和渲染检查通过；全量测试存在未通过项。
- 已处理 `main`（`2c7c487`）合入 `feat/own` 的冲突及恢复流程死锁，相关回归通过；尚未创建合并提交。
- 本文件从本次任务开始记录，其他项目进度待确认。

## 已完成

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

## 最近验证

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
