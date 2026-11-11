# Design: fun-salary-home-entry

## Context

首页已有 `.funentry` 突出入口卡（fun-compass C2，骨白+黄铜描边，`v-if="funEnabled"`）。本变更复用同一视觉加第二张卡。

## Goals / Non-Goals

**Goals:** 最小 diff 加卡，样式零新增依赖（复用 .funentry）。
**Non-Goals:** 重排首页其他区块；改快捷 pill；新样式体系。

## Decisions

- **堆叠而非并排**：第二张卡完整复用 `.funentry` 类（同尺寸同边距），仅 `+.funentry` 补 `margin-top:10px`——保住「突出入口」的既定观感，改动最小；并排半宽卡会推翻已验收的 funentry 形态（弃）。
- **图标**：`¥` 字符（Oswald 黄铜、`.funentry-y`），与【我的】分组内工资行的 ¥ tile 语义一致；碳图标备选（弃：文字更直白）。
- **门控**：沿用 `funEnabled`（与方位卡同源），双卡同隐同显。
- **函数**：`goSalary()` → `router.push('/pages/fun/salary/index')`，紧挨 `goFunCompass`。

## Risks / Trade-offs

- [首页纵向变长] 两张入口卡 +86px，快捷 pill 与广告预留区下移——广告区尚未实现（注释占位），无实际冲突。

## Migration Plan

纯增量模板+样式，revert 即回滚。

## 验证方案

lint / type-check / build:h5 / build:mp-weixin；H5 点击直达与返回（Playwright 可用时）；开关双隐静态复核（v-if 同源）；`openspec validate`。
