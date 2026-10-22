# Apple HIG 合规审查 — 「牌面」方向

> 审查对象：[design-onepager-paimian.md](design-onepager-paimian.md)（5 页移动端设计）
> 平台：iOS（390×844，iPhone 14/15 逻辑分辨率）· 依据：Apple Human Interface Guidelines 公开规范要点
> 说明：上游 apple-hig-skills bundle 未安装（用户跳过），本审查基于内置 HIG 知识，非逐条引用原文

## 结论摘要

**可进入视觉稿阶段**。必改 2 项（🔴）、实施期注意 6 项（🟡）、合规 5 项（🟢）。

---

## 🔴 必改项（进 Figma / HTML 前修正）

### 1. 小字号红字对比度不足
- **现状**：漆红 `#D23B22` 用于牌面白底上的金额与 caption；实测对比度 ≈ 4.7:1
- **HIG**：WCAG AA（HIG Accessibility 明确要求）——**正文/小字需 ≥ 4.5:1**，4.7 对 48pt 大数字绰绰有余，但 11–15pt 的 caption/label 处于临界，且"增强对比度"模式下不达标
- **修正**：拆分双值——`vermillion/500 #D23B22`（仅限 ≥24pt 数字与图形）+ `vermillion/600 #B32D18`（≤15pt 文字，对比度 ≈ 6.1:1）。靛蓝 `#3D5A96` 同理核验小字场景（≈ 5.9:1，达标，记录备查）

### 2. 底部安全区
- **现状**：拇指区 CTA 的 `paddingBottom` 只写了 24–28
- **HIG**：必须避让 Home Indicator 安全区（34pt）
- **修正**：所有含底部 CTA/TabBar 的页面，容器 `padding-bottom = 34 + 16`；TabBar 自身高度 = 内容 49 + 安全区 34（当前 74 需确认是否已含，HTML/HTML 稿按 49+34=83 校正）

---

## 🟡 实施期注意（记录进规范，不阻塞视觉稿）

### 3. 字号下限与 Dynamic Type
- 我们的 `caption 11` = HIG Footnote 下限，**允许但不可再低**；`body 15` 小于 iOS 默认 Body 17——对"牌谱表格"这类信息密集场景可接受
- 实现期必须接入 Dynamic Type（`UIFontMetrics` 缩放），**Oswald 展示数字除外**（display 数字锁定尺寸是合理例外，HIG 允许展示级文本不随动态字体缩放）
- 设计侧动作：在 design-rules.md 标注"11pt 为绝对下限"

### 4. 图标用 SF Symbols（实现期）
- TabBar 的首页/战绩图标当前为手绘 SVG
- **HIG**：TabBar 图标应使用 **SF Symbols**（`house` / `chart.bar`），保证与系统字重、网格、深浅色模式自动适配
- 设计稿保留现 SVG 作视觉参考；实现清单标注"替换为 SF Symbols fill 配置"

### 5. Reduce Motion 降级
- 翻牌结算（300ms）、筹码浮标、盖章动效必须提供 `prefers-reduced-motion` 降级：翻牌 → 淡入 100ms；浮标 → 直接显示结果色；盖章 → 静态印章
- HIG Accessibility → Motion：**用户要求减少动态时，不得有非必要动效**

### 6. 反馈：触感与 Toast 位置
- ±记分 = 系统 `UIImpactFeedbackGenerator(.light)`；盖章 = `.notificationOccurred(.success)`（实现期）
- Toast/「已记入牌谱」应出现在**顶部安全区之下**，不得盖住底部 TabBar 或 CTA（HIG banners 模式）

### 7. 导航结构核验（结论：符合）
- TabBar 仅存在于两个根页面（首页/战绩）✓ —— 符合 HIG Tab Bar 用于**平级根级目的地**的定位
- 开局/记分/结算为 push 栈（带 `‹` 返回）✓
- **记分页无返回键是有意为之**：进行中的牌局退出属破坏性操作，入口收敛到明确的 `结束牌局`（并应在实现期加确认弹窗 → 符合 HIG 谨慎操作须确认）
- 结算页建议：保存成功后 pop 回首页而非留在栈内（HIG：任务完成后返回发起上下文）

### 8. 深色规范
- 底色 `#14120F` 非纯黑 ✓（HIG：避免纯黑，防止 OLED 拖影与层级丢失）
- 未使用系统语义色的问题：自定义色板需在实现期提供 `dark` 语义映射，保证系统级"增强对比度""智能反转"下文本仍可读

---

## 🟢 已合规项

| 项 | 现状 |
|---|---|
| 最小触达区 44×44 | ± 筹码钮 56 ✓，stepper 44 ✓（恰好达标），CTA 56 ✓ |
| 颜色不可单独承载信息 | 所有金额带 +/−/0 符号 ✓ |
| 文案 sentence case、无全大写 | ✓（中文无此问题，英文/数字标签已检查） |
| 主操作拇指热区（下 1/3） | ✓ 全部 CTA 底部横排 |
| 状态栏/刘海适配 | 设计含状态栏行（HTML 稿将含 Dynamic Island 区域留白） |

---

## 修正落地清单

| # | 动作 | 落到哪 |
|---|---|---|
| 1 | 新增 `vermillion/600 #B32D18` 小字专用红 | 详案 token 表 + 未来 Figma 变量 |
| 2 | 底部安全区 34pt 写入布局规则 | 详案 5 页逐屏 + design-rules.md（Figma 恢复后） |
| 3 | caption 11pt = 绝对下限 | design-rules.md |
| 4 | SF Symbols 替换清单 | 本文件（实现期） |
| 5 | Reduce Motion 降级表 | 本文件 #5 |
