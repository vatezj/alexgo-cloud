# Figma 续作脚本（打牌记账 App）

额度恢复或升级套餐后，把这些脚本**整文件内容**粘贴进 `use_figma` 的 `code` 参数即可（`fileKey: dHMgzmrrWJ8ztjWytPXdIW`）。

## 执行顺序（严格按序，每个一次调用）

| # | 文件 | 目标页 | 说明 |
|---|---|---|---|
| 1 | `02-setup.js` | 📱 Screens `0:1` | 开局设置页（bug 已修） |
| 2 | `03-score.js` | 📱 Screens `0:1` | 记分页（核心）+ 加分动效标注 |
| 3 | `04-settle.js` | 📱 Screens `0:1` | 结算页 |
| 4 | `05-stats.js` | 📱 Screens `0:1` | 战绩统计页 |
| 5 | `06-foundations.js` | 🎨 Foundations `14:72` | 设计规范可视化页 |
| 6 | 截图校验 | — | `get_screenshot` nodeId=`0:1` 与 `14:72`，各 1 次 |

共需 **8 次** MCP 调用（含 1–2 次修复）。之后把 CLAUDE.md 中"待办"改为完成。

## ID 账本

见 `/tmp/design-system-state-paijv.json`；组件/样式 ID 已内嵌在各脚本中。

## 注意

- 每次 `use_figma` 只切一次页面；不要并行调用
- 脚本按"自包含"编写（含全部 helper），直接整段粘贴
- 若返回 rate limit，等下月重置或升级 Dev/Full 席位
