# Design: fun-compass-off

## Context

审核拒绝针对算卜功能链；`fun_compass_enable` 默认 true 且只藏入口，分包代码仍进包。A（打包剔除）+ B（删因果链）一并做，避免「藏了但还在包里」。

## Decisions

### D1 删除清单（端侧）

| 路径 | 动作 |
|------|------|
| `src/pages/fun/compass/`（6 页） | 删 |
| `src/utils/funCompass/`（copy、zhiwei） | 删 |
| `src/store/funCompass/`（archives、recommendation） | 删 |
| `src/composables/funCompass/` | 删 |
| `src/components/funCompass/ModDisclaimer.vue` | 删（仅 compass 用） |
| `src/components/funCompass/ModNav.vue` | **留**（salary） |
| `src/config/tools.ts` 中 zhiwei/ZHIWEI | 删端点与说明；无其他消费方则整文件可留空壳或删——无引用则删文件 |
| `.env.development/.production/.staging` 的 `VITE_ZHIWEI_BASE_URL` | 删 |

### D2 入口

- `index.vue`：删趣味方位 funentry 卡与 `goFunCompass`；工资卡去掉 `funEnabled` 门控（恒显或删 v-if）；删 `useFunCompassConfig` import。
- `mine.vue`：删趣味方位行与 `goCompass`；分组 `v-if="enabled"` 去掉（工资仍在组内）；`MINE_GROUP_FOOT` 改为不含「档案/民俗方位」的中性脚注（如「小工具仅供娱乐」）或删脚注。

### D3 分包

删除 compass 页后 `UniHelperPages` 仍扫 `src/pages/fun`，仅剩 salary → `pages.json` subPackages root=pages/fun 只含 salary。验证：`build:mp-weixin` 后读 `dist/build/mp-weixin/pages.json`。

### D4 后端

本轮不删 `plugin/zhiwei` 与网关 `/api/calc`（非小程序包内容）。若后续要彻底下线服务另开变更。

## Risks

- 漏删 import → type-check 红：以 `pnpm type-check` 为门禁。
- 分享卡配置仍有 `simulator` 键：无页面消费，无害，不改 sharecfg 服务端。
