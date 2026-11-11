# Design: fun-salary-confirm-flow

## Context

现页面选值即算、结果常显、带默认值；用户要求改为「空态→填写→确认→折叠摘要+结果」两态机。

## Goals / Non-Goals

**Goals:** 一个明确的确认步骤与两态 UI；结果只反映确认过的值。
**Non-Goals:** 改口径、改样式体系、动其他页。

## Decisions

- **状态机** `phase: 'input' | 'result'`：input = 表单展开、结果区不渲染（空态与编辑态同构）；result = 摘要行 + 结果区。初始 `input` 且字段全空（去默认值）。
- **确认值分离**：新增 `confirmed: SalaryParams | null` + `rates`；`onConfirm` 校验齐全与 `calcRates` 非 null 后写入并 `rebase()`、`phase='result'`。**删除原 watch(paramsKey) 自动重算**——编辑期间不碰结果，杜绝「表单值≠展示值」。
- **摘要行**：`¥15,000 · 09:00–12:00 · 13:30–18:00` 单行（12.5px，超长省略）+ 右侧 `i-carbon-chevron-down`；点击整行 → `phase='input'`。
- **占位**：触发区空值显「请选择」/「--:--」（dim 色 class），有值后回显原样式。
- **确认按钮**：输入卡底部全宽骨白胶囊「确认入账」（卡内深底上用骨白=主按钮层级）；始终可点，靠 toast 引导（与项目 toast 惯例一致，不用 disabled 态）。
- **计时器**：interval 保留，tick/rebase 以 `confirmed/rates` 为基（null 时跳过）；跨日守卫沿用。
- **弃**：结果区编辑期间常显（易误读）；defaults 保留（违背空态要求）。

## Risks / Trade-offs

- [摘要行过长] 390 宽下三段可能挤 → 12.5px + nowrap + ellipsis；如仍挤可拆两行（用户明确要一行，先按一行）。
- [空态首屏偏空] 可接受——交互本就引导填写。

## Migration Plan

纯交互重排，无数据迁移；revert 即回滚。

## 验证方案

lint/type-check/build:h5/mp；Playwright 可用时 e2e 五场景（空态/确认折叠/再展开/倒挂拦截/字段不全），断线则列人工清单；`openspec validate`。

---

## 修订轮决策（2026-11-09 用户三点补充）

- **布局根因**：`.fcn-nav/.sal-content/.sal-footer` 共享 `width:100%` 但 content/footer 各带 `padding …16px`，且均为 content-box（`box-sizing` 不继承，`.page-wraper` 的 border-box 管不到）→ 窄屏总宽 = 100%+32px 溢出。**修法**：共享规则补 `box-sizing: border-box`（H5 390px 实测 scrollWidth=390 零溢出；宽屏 480 居中不回归）。compass 页无此病是因为 padding 在自身且已 border-box。
- **月薪双通道（用户选定）**：原生 `input type="number"`（沿 bazi `.fc-input` 深色胶囊样式语言，前缀 `sal-`）+ 右侧「选」小胶囊开既有 wd-picker；picker `v-model` 改为独立 `salaryPickV`，确认后回填 `salaryStr`；输入框是唯一数据源（`draft` 读它）。**弃**：弹层内加 tab（多一步）、双胶囊并排（视觉碎）。
- **默认值（用户选定）**：`salaryStr='15000'`、四时间默认 09:00/12:00/13:30/18:00。空态占位样式保留给「月薪被清空」场景（placeholder「请输入」）；时间胶囊占位逻辑可简化（恒有值，保留无害）。
- **校验口径**：`draft` 对月薪要求 `Number(salaryStr) > 0` 且非 NaN；空/0/负 → onConfirm toast「请输入工资」。时间仍靠 calcRates 倒挂拦截。spec 场景「字段不全拦截」改名「月薪清空拦截」（时间恒有值，不再可能不全）。
- **不变**：确认折叠两态机、confirmed 分离、摘要行、tick/rebase、计算口径。
