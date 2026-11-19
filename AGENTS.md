# Kero 项目规范

- 本项目使用 Go 和 MyGo 原生 UI；沿用现有目录结构、组件和主题机制。
- 根目录存放桌面应用，`internal/` 存放内部协议、守护进程和 CLI，`locales/` 存放国际化文案，`icons/` 存放图标。
- 开始工作前阅读本文件和 `ROADMAP.md`；MyGo 修改同时遵循 `.agents/skills/mygo-maintenance/SKILL.md`。
- 只修改任务相关内容，不引入无关重构或依赖。UI 元素不得跨构建保存，绘制回调只负责绘制。
- UI 修改运行相关回归测试、`go tool mygo vet .` 和 `CGO_ENABLED=0 go test ./...`，并检查渲染结果；明确记录未通过项和验证限制。
- 开发完成后同步 `ROADMAP.md`，只有实现并验证的事项可以标记完成。
- 发布遵循 `RELEASING.md`；删除、回滚、推送、发布及敏感配置修改须事先征得用户同意。

