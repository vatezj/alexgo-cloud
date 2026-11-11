# Tasks: fun-compass-review-copy

## 1. 前端文案

- [x] 1.1 `copy.ts`：按 design D1 改 LOCAL_ONLY / ARCHIVE_EMPTY_HINT / ABOUT_PARAGRAPHS（涉排盘/生辰/五行句）/ CLEAR_HINT / HOME_LOCAL_DOT 等；新增 `REVIEW_RISK_WORDS`；文件头锁定注释改为「合规锁定，上架口径」
- [x] 1.2 六页 + 入口：`index.vue`（compass 模块）、`bazi.vue`、`simulator.vue`、`archive.vue`、`about.vue`、`compass.vue`（映射条与说明 toast）、`mine/index.vue`、`home/index.vue` 按 D1 替换用户可见字符串（注释可保留历史说明，但新增注释不再写八字/排盘）
- [x] 1.3 `zhiwei.ts`：错误消息「排盘…」→「测算…」（请求失败/服务异常/格式异常）

## 2. 后端 reason（只文案）

- [x] 2.1 `modules/scorecard/plugin/zhiwei/src/services/scoring.js`：六处 reason 按 D2 替换；计分逻辑零改动
- [x] 2.2 `test/scoring.test.js`：reason 断言改为新文案；分值断言不动；`npm test` 全绿

## 3. 验证

- [x] 3.1 `pnpm lint` + `pnpm type-check` + `pnpm build:h5`（uniapp 目录）
- [x] 3.2 黑名单 grep：`REVIEW_RISK_WORDS` ∪ {生辰,五行} 于 `src/pages/fun/compass`、`src/pages/mine/index.vue`、`src/pages/index/index.vue`（用户可见模板区）、`src/utils/funCompass` → 模板/常量 0 命中
- [x] 3.3 zhiwei `npm test` 复跑；`openspec validate fun-compass-review-copy` 通过
