以下是 **路线 B（后端 Node.js 计算）** 的具体实现方案，覆盖从项目初始化、核心排盘调用、评分引擎、API。

---

## 📁 项目初始化与技术栈

**技术栈**：Node.js 20+、Express、`iztro`、`@spensercai/qimen-rs`，

```bash
mkdir mahjong-seat-api && cd mahjong-seat-api
npm init -y
npm install express cors iztro @spensercai/qimen-rs
```

`qimen-rs` 的 Node.js 绑定要求 **Node.js 20+**，安装后通过 `import { calculate } from '@spensercai/qimen-rs'` 调用。`iztro` 同样直接引入 `astro` 对象即可使用。

项目目录结构：

```
mahjong-seat-api/
├── src/
│   ├── index.js              # Express 入口
│   ├── routes/
│   │   └── calc.js           # POST /api/calc
│   ├── services/
│   │   ├── ziwei.js          # 紫微排盘封装
│   │   ├── qimen.js          # 奇门起局封装
│   │   └── scoring.js        # 评分引擎
│   ├── constants/
│   │   ├── starTypes.js      # 吉星/煞星分类表
│   │   └── mappings.js       # 九宫→方位、地支→方位映射
│   └── utils/
│       └── time.js           # 时间归一化
├── Dockerfile
└── package.json
```


## 🪐 紫微排盘封装（ziwei.js）

`iztro` 的 `astro.bySolar(dateStr, timeIndex, gender, fixLeap, language)` 是核心调用方式。`timeIndex` 取值 **0–12**（0=早子时，1=丑时……12=晚子时），`fixLeap` 控制闰月处理，语言可传 `'zh-CN'`。

```javascript
// src/services/ziwei.js
import { astro } from 'iztro';

export function getZiweiPalaces({ birthDate, birthHourIndex, gender }) {
  const astrolabe = astro.bySolar(
    birthDate,          // 'YYYY-MM-DD'
    birthHourIndex,     // 0–12
    gender === 'female' ? '女' : '男',
    true,               // 启用闰月修正
    'zh-CN',
  );

  return astrolabe.palaces.map((p) => ({
    name: p.name,                  // 宫名，如 '财帛宫'
    earthlyBranch: p.earthlyBranch, // 地支，如 '子'
    stars: p.majorStars.concat(p.minorStars).map((s) => ({
      name: s.name,
      brightness: s.brightness,    // 庙旺利陷，可作为额外信号
    })),
  }));
}
```

**关于真太阳时**：`iztro` 本身不直接接受经度参数做真太阳时校正，真太阳时的计算需要在前端或独立的工具函数中完成——按“时区 + 经度修正（每度 4 分钟）+ 均时差”三步将钟表时间转为真太阳时，再将校正后的时间传入 `iztro`。如果产品初期不做真太阳时，直接在表单中让用户选择出生时辰序号即可，并在结果页注明“未启用真太阳时校正”。


## ⛩️ 奇门起局封装（qimen.js）

`qimen-rs` 的 Node.js 接口输入是**公历年月日时分**，默认 UTC 偏移 `+480` 分钟，默认 **23:00 子初换日**，**不读取机器时区**。因此 `playTime` 必须由前端显式传递用户所在时区的本地时间，或统一传 ISO 8601 字符串并在后端解析。

```javascript
// src/services/qimen.js
import { calculate } from '@spensercai/qimen-rs';

export function getQimenPalaces({ playTime }) {
  const dt = new Date(playTime); // playTime: '2026-11-05T20:30:00+08:00'

  const result = calculate({
    year: dt.getFullYear(),
    month: dt.getMonth() + 1,
    day: dt.getDate(),
    hour: dt.getHours(),
    minute: dt.getMinutes(),
    utcOffset: 480, // 东八区；若面向多时区用户，从前端传入
  });

  // result.palaces 为九宫数组，需适配为内部统一结构
  return result.palaces.map((p) => ({
    palaceIndex: p.palaceIndex, // 1–9
    door: p.door,               // 八门名称，如 '生门'
    star: p.star,               // 九星
    deity: p.deity,             // 八神
    // 可选：旺衰、三奇等字段，视 qimen-rs 返回结构而定
  }));
}
```

**关键适配点**：`qimen-rs` 默认采用 **时家奇门、拆补法、转盘、中五寄坤、天禽随芮**。建议在后端加一层**适配层**，将原始返回值转换为内部统一的 `{ palaceIndex, door, star, deity }` 结构，避免库的字段命名变化渗透到评分逻辑中。


## 🧮 评分引擎（scoring.js）

评分引擎是纯函数，接收紫微和奇门的结构化数据，输出排序后的推荐方位列表。

### 🗺️ 常量映射

```javascript
// src/constants/mappings.js
export const QIMEN_PALACE_TO_DIRECTION = {
  1: '北',    // 坎宫
  2: '西南',  // 坤宫
  3: '东',    // 震宫
  4: '东南',  // 巽宫
  6: '西北',  // 乾宫
  7: '西',    // 兑宫
  8: '东北',  // 艮宫
  9: '南',    // 离宫
  // 中五宫无门，不参与推荐
};

export const ZIWEI_BRANCH_TO_DIRECTION = {
  子: '北', 丑: '东北', 寅: '东北', 卯: '东',
  辰: '东南', 巳: '东南', 午: '南', 未: '西南',
  申: '西南', 酉: '西', 戌: '西北', 亥: '西北',
};
```

### ⭐ 吉星/煞星分类表

紫微斗数传统分类中，**六吉星**为左辅、右弼、天魁、天钺、文昌、文曲；**六煞星**为擎羊、陀罗、火星、铃星、地空、地劫。禄存、天马、四化（化禄/权/科/忌）也建议纳入分类表。财帛宫遇吉星庙旺守值（如紫微、天府、禄存、化禄、左辅、右弼等）主财多，遇擎羊等煞星则财源不稳。

```javascript
// src/constants/starTypes.js
export const LUCKY_STARS = new Set([
  '左辅', '右弼', '天魁', '天钺', '文昌', '文曲',
  '禄存', '天马', '化禄', '化权', '化科',
]);

export const MALEFIC_STARS = new Set([
  '擎羊', '陀罗', '火星', '铃星', '地空', '地劫', '化忌',
]);
```

### ⚖️ 评分函数

生门旺衰在九宫中有明确规则：**艮宫、坤宫为旺，离宫为相，乾宫、兑宫休，坎宫囚，震宫、巽宫死**。据此在评分中加权。

```javascript
// src/services/scoring.js
import {
  QIMEN_PALACE_TO_DIRECTION,
  ZIWEI_BRANCH_TO_DIRECTION,
} from '../constants/mappings.js';
import { LUCKY_STARS, MALEFIC_STARS } from '../constants/starTypes.js';

const SHENGMEN_WANG_SHUAI = {
  艮宫: 15, 坤宫: 15, 离宫: 8,   // 旺 / 相
  乾宫: 0,  兑宫: 0,              // 休
  坎宫: -5,                       // 囚
  震宫: -10, 巽宫: -10,           // 死
};

export function scoreSeats(qimenPalaces, ziweiPalaces) {
  const scores = {};
  for (const dir of Object.values(QIMEN_PALACE_TO_DIRECTION)) {
    scores[dir] = { score: 0, reasons: [] };
  }

  // ---- 奇门维度 ----
  const shengmen = qimenPalaces.find((p) => p.door === '生门');
  if (shengmen) {
    const dir = QIMEN_PALACE_TO_DIRECTION[shengmen.palaceIndex];
    if (dir && scores[dir]) {
      scores[dir].score += 50;
      scores[dir].reasons.push('奇门生门落此方，主生机与财气');

      const palaceName = shengmen.palaceName; // 需适配层提供宫名
      const wangShuai = SHENGMEN_WANG_SHUAI[palaceName];
      if (wangShuai !== undefined) {
        scores[dir].score += wangShuai;
        if (wangShuai > 0) scores[dir].reasons.push(`生门在${palaceName}旺相`);
        if (wangShuai < 0) scores[dir].reasons.push(`生门在${palaceName}衰囚`);
      }
    }
  }

  // ---- 紫微维度 ----
  const TARGET_PALACES = new Set(['命宫', '财帛宫', '迁移宫']);
  for (const zp of ziweiPalaces) {
    const dir = ZIWEI_BRANCH_TO_DIRECTION[zp.earthlyBranch];
    if (!dir || !scores[dir] || !TARGET_PALACES.has(zp.name)) continue;

    const lucky = zp.stars.filter((s) => LUCKY_STARS.has(s.name));
    const malefic = zp.stars.filter((s) => MALEFIC_STARS.has(s.name));

    if (lucky.length >= 2) {
      scores[dir].score += 25;
      scores[dir].reasons.push(
        `紫微${zp.name}吉星聚集（${lucky.map((s) => s.name).join('、')}）`,
      );
    }
    if (malefic.length >= 2) {
      scores[dir].score -= 15;
      scores[dir].reasons.push(
        `${zp.name}煞星较多（${malefic.map((s) => s.name).join('、')}），建议避开`,
      );
    }
  }

  return Object.entries(scores)
    .map(([direction, v]) => ({
      direction,
      score: v.score,
      reason: v.reasons.length ? v.reasons.join('；') : '无显著吉凶信号',
    }))
    .filter((r) => r.score > 0) // 只返回正分方位，避免推荐“煞方”
    .sort((a, b) => b.score - a.score)
    .slice(0, 3);
}
```


## 🔌 API 接口设计（routes/calc.js）

**请求**：`POST /api/calc`

```json
{
  "birthDate": "2000-08-16",
  "birthHourIndex": 2,
  "gender": "female",
  "playTime": "2026-11-05T20:30:00+08:00"
}
```

**响应**：

```json
{
  "recommendations": [
    {
      "rank": 1,
      "direction": "东北",
      "score": 75,
      "reason": "奇门生门落艮宫旺相；紫微财帛宫吉星聚集（禄存、天马）"
    },
    {
      "rank": 2,
      "direction": "西南",
      "score": 65,
      "reason": "奇门生门落坤宫旺相"
    }
  ],
  "meta": {
    "qimenConvention": "时家奇门·拆补法·转盘·中五寄坤",
    "dayBoundary": "23:00子初换日",
    "trueSolarTime": false
  },
  "disclaimer": "仅供娱乐参考，请勿沉迷赌博。"
}
```

**路由实现**：

```javascript
// src/routes/calc.js
import { Router } from 'express';
import { getZiweiPalaces } from '../services/ziwei.js';
import { getQimenPalaces } from '../services/qimen.js';
import { scoreSeats } from '../services/scoring.js';

const router = Router();

router.post('/calc', (req, res) => {
  try {
    const { birthDate, birthHourIndex, gender, playTime } = req.body;

    if (!birthDate || birthHourIndex == null || !gender || !playTime) {
      return res.status(400).json({ error: '缺少必要参数' });
    }

    const ziwei = getZiweiPalaces({ birthDate, birthHourIndex, gender });
    const qimen = getQimenPalaces({ playTime });
    const recommendations = scoreSeats(qimen, ziwei);

    res.json({
      recommendations,
      meta: {
        qimenConvention: '时家奇门·拆补法·转盘·中五寄坤',
        dayBoundary: '23:00子初换日',
        trueSolarTime: false,
      },
      disclaimer: '仅供娱乐参考，请勿沉迷赌博。',
    });
  } catch (err) {
    console.error(err);
    res.status(500).json({ error: '排盘计算失败，请检查输入时间格式' });
  }
});

export default router;
```

**Express 入口**：

```javascript
// src/index.js
import express from 'express';
import cors from 'cors';
import calcRouter from './routes/calc.js';

const app = express();
app.use(cors());
app.use(express.json());
app.use('/api', calcRouter);

const port = process.env.PORT || 80;
app.listen(port, () => console.log(`Server running on port ${port}`));
```



## ⚠️ 实操中需要处理的细节

**1. 时间语义必须在前后端对齐**。`qimen-rs` 明确“输入是指定偏移下的民用时间，不读取机器时区”。前端传 `playTime` 时应传带时区偏移的 ISO 8601 字符串（如 `+08:00`），后端用 `new Date()` 解析后再取年月日时分，避免服务器时区干扰。

**2. 生门旺衰依赖宫名，适配层必须提供 `palaceName`**。`qimen-rs` 的返回值中需要确认是否直接包含宫名（如“艮宫”），如果只有 `palaceIndex`，则由适配层通过 `PALACE_INDEX_TO_NAME` 映射表补全，再传入评分引擎。

**3. 吉星/煞星分类表是产品核心配置，建议外置为 JSON**，初期覆盖六吉、六煞、禄存、天马、四化即可，后续可按用户反馈迭代，无需重新部署代码（如果分类表放在环境变量或数据库）。

**4. 缓存策略**：同一时辰（2 小时内）的奇门盘是固定的，可以按 `时辰key` 缓存 `qimen` 结果，减少重复计算。紫微盘按用户出生信息缓存，粒度更粗，收益不大，初期可以不缓存。

**5. 免责声明固定展示**。结果页必须包含“**仅供娱乐参考，请勿沉迷赌博**”，并在产品文案中引导娱乐和社交属性。
