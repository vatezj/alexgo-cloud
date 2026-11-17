import { NUM, SERIF, SANS, T } from "../theme";
import { ComplianceStrip, Score } from "../ui";

/**
 * 小红书图文卡片（3:4 · 1080×1440 · 静态帧）
 * 墨金设计语言与视频版一致；合规口径与小红书版视频一致
 * （无二维码、无站外引流词）。
 */

const W = 1080;
const H = 1440;

/** 卡片底 + 页脚（左品牌 / 右页码） */
const Shell: React.FC<{ page: number; total?: number; children: React.ReactNode }> = ({
  page,
  total = 8,
  children,
}) => (
  <div
    style={{
      width: W,
      height: H,
      backgroundColor: T.ink900,
      backgroundImage: T.spot,
      fontFamily: SANS,
      color: T.onInk,
      position: "relative",
      overflow: "hidden",
      boxSizing: "border-box",
    }}
  >
    {children}
    <div
      style={{
        position: "absolute",
        left: 60,
        right: 60,
        bottom: 46,
        display: "flex",
        justifyContent: "space-between",
        fontSize: 26,
        letterSpacing: "0.14em",
        color: T.dim,
      }}
    >
      <span style={{ fontFamily: SERIF, color: T.brass, fontWeight: 700 }}>
        记账局叙
      </span>
      <span>
        {page} / {total}
      </span>
    </div>
  </div>
);

/** 大标题（serif + 可选 brass 强调行） */
const Title: React.FC<{
  lines: { text: string; accent?: boolean }[];
  size?: number;
  top?: number;
}> = ({ lines, size = 88, top = 110 }) => (
  <div
    style={{
      position: "absolute",
      top,
      left: 0,
      right: 0,
      textAlign: "center",
      fontFamily: SERIF,
      fontWeight: 700,
      lineHeight: 1.35,
      letterSpacing: "0.08em",
    }}
  >
    {lines.map((l) => (
      <div
        key={l.text}
        style={{
          fontSize: size,
          color: l.accent ? T.brass : T.onInk,
          textShadow: l.accent ? `0 4px 24px ${T.brassA50}` : "none",
        }}
      >
        {l.text}
      </div>
    ))}
  </div>
);

/** 骨白信息卡（tile 风） */
const TileCard: React.FC<{
  children: React.ReactNode;
  style?: React.CSSProperties;
}> = ({ children, style }) => (
  <div
    style={{
      background: `linear-gradient(165deg, ${T.tile}, ${T.tileShade})`,
      boxShadow: T.shadowTile,
      borderRadius: 24,
      ...style,
    }}
  >
    {children}
  </div>
);

/* ───────── 01 封面 ───────── */

const MiniCard: React.FC<{
  x: number;
  y: number;
  rot: number;
  rank: string;
  suit: string;
  red?: boolean;
}> = ({ x, y, rot, rank, suit, red }) => (
  <div
    style={{
      position: "absolute",
      left: x,
      top: y,
      width: 150,
      height: 210,
      borderRadius: 16,
      background: `linear-gradient(160deg, ${T.tile}, ${T.tileShade})`,
      boxShadow: T.shadowTile,
      transform: `rotate(${rot}deg)`,
      padding: "12px 14px",
      boxSizing: "border-box",
      fontFamily: SERIF,
      fontWeight: 700,
      color: red ? "#B32D18" : T.jet,
    }}
  >
    <div style={{ fontSize: 32, lineHeight: 1 }}>{rank}</div>
    <div style={{ fontSize: 26, lineHeight: 1.2 }}>{suit}</div>
    <div
      style={{
        position: "absolute",
        right: 14,
        bottom: 10,
        fontSize: 56,
        opacity: 0.9,
      }}
    >
      {suit}
    </div>
  </div>
);

export const Card01Cover = () => (
  <Shell page={1}>
    {/* 顶部品类标签 */}
    <div
      style={{
        position: "absolute",
        top: 120,
        left: 0,
        right: 0,
        display: "flex",
        justifyContent: "center",
      }}
    >
      <div
        style={{
          padding: "14px 40px",
          borderRadius: 999,
          border: `1.5px solid ${T.brassA50}`,
          color: T.brass,
          fontSize: 30,
          letterSpacing: "0.3em",
        }}
      >
        打牌记账小程序
      </div>
    </div>

    <Title
      lines={[{ text: "散场了" }, { text: "谁输谁赢？", accent: true }]}
      size={132}
      top={300}
    />
    <div
      style={{
        position: "absolute",
        top: 640,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 42,
        letterSpacing: "0.16em",
        color: T.dim,
      }}
    >
      这账，交给小程序来记
    </div>

    {/* 牌桌 + 散牌装饰 */}
    <div
      style={{
        position: "absolute",
        left: "-12%",
        right: "-12%",
        bottom: -300,
        height: 900,
        borderRadius: "50%",
        background: `radial-gradient(ellipse at 50% 28%, ${T.feltHi}, ${T.felt} 55%, ${T.feltDeep} 100%)`,
        boxShadow: "inset 0 40px 80px rgba(0,0,0,0.5)",
        border: `5px solid ${T.brassLo}`,
      }}
    />
    <MiniCard x={90} y={1080} rot={-13} rank="K" suit="♠" />
    <MiniCard x={310} y={1130} rot={7} rank="7" suit="♥" red />
    <MiniCard x={530} y={1085} rot={-4} rank="A" suit="♣" />
    <MiniCard x={750} y={1140} rot={14} rank="Q" suit="♦" red />
    <div
      style={{
        position: "absolute",
        top: 880,
        left: 0,
        right: 0,
        textAlign: "center",
        fontFamily: SERIF,
        fontWeight: 700,
        fontSize: 54,
        letterSpacing: "0.2em",
        color: T.brass,
        textShadow: `0 4px 24px ${T.brassA50}`,
      }}
    >
      散场不散账，一键算清
    </div>
  </Shell>
);

/* ───────── 02 痛点 ───────── */

const PAIN_ROWS = [
  { n: "01", t: "群里掰扯半小时", d: "谁也不认谁的数，越算越乱" },
  { n: "02", t: "记分纸随手一塞", d: "回头再找，早不知道去哪了" },
  { n: "03", t: "翻半天聊天记录", d: "才勉强凑出个大概" },
];

export const Card02Pain = () => (
  <Shell page={2}>
    <Title
      lines={[{ text: "散场后的" }, { text: "名场面", accent: true }]}
      size={96}
      top={140}
    />
    <div
      style={{
        position: "absolute",
        top: 430,
        left: 70,
        right: 70,
        display: "flex",
        flexDirection: "column",
        gap: 34,
      }}
    >
      {PAIN_ROWS.map((r) => (
        <TileCard key={r.n} style={{ padding: "44px 48px" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 34 }}>
            <div
              style={{
                fontFamily: NUM,
                fontSize: 76,
                fontWeight: 600,
                color: T.brassLo,
                lineHeight: 1,
              }}
            >
              {r.n}
            </div>
            <div>
              <div
                style={{
                  fontFamily: SERIF,
                  fontWeight: 700,
                  fontSize: 44,
                  color: T.jet,
                  letterSpacing: "0.04em",
                }}
              >
                {r.t}
              </div>
              <div
                style={{
                  fontSize: 30,
                  color: "rgba(42,39,34,0.55)",
                  marginTop: 10,
                }}
              >
                {r.d}
              </div>
            </div>
          </div>
        </TileCard>
      ))}
    </div>
    <div
      style={{
        position: "absolute",
        top: 1200,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 36,
        letterSpacing: "0.12em",
        color: T.dim,
      }}
    >
      不是小气，是真记不清
    </div>
  </Shell>
);

/* ───────── 03 三步用法 ───────── */

const STEPS = [
  { n: "01", t: "我要开房", d: "建房拉好友，扫码就能进" },
  { n: "02", t: "随手记一笔", d: "每一手记分，全桌实时同步" },
  { n: "03", t: "一键结清", d: "谁记给谁多少，一眼看清" },
];

export const Card03Steps = () => (
  <Shell page={3}>
    <Title
      lines={[{ text: "三步，" }, { text: "账就清了", accent: true }]}
      size={96}
      top={140}
    />
    <div
      style={{
        position: "absolute",
        top: 430,
        left: 70,
        right: 70,
        display: "flex",
        flexDirection: "column",
        gap: 40,
      }}
    >
      {STEPS.map((s) => (
        <div
          key={s.n}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 40,
            padding: "40px 48px",
            borderRadius: 24,
            background: T.ink700,
            border: `1.5px solid ${T.onInkA28}`,
          }}
        >
          <div
            style={{
              width: 110,
              height: 110,
              borderRadius: "50%",
              flex: "none",
              background: `radial-gradient(circle at 32% 28%, ${T.brassHi}, ${T.brass} 62%, ${T.brassLo})`,
              border: `6px dashed rgba(20,18,15,0.35)`,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontFamily: NUM,
              fontSize: 52,
              fontWeight: 600,
              color: T.jet,
            }}
          >
            {s.n}
          </div>
          <div>
            <div
              style={{
                fontFamily: SERIF,
                fontWeight: 700,
                fontSize: 46,
                letterSpacing: "0.1em",
                color: T.onInk,
              }}
            >
              {s.t}
            </div>
            <div style={{ fontSize: 31, color: T.dim, marginTop: 8 }}>
              {s.d}
            </div>
          </div>
        </div>
      ))}
    </div>
    <div
      style={{
        position: "absolute",
        top: 1215,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 34,
        letterSpacing: "0.1em",
        color: T.brass,
      }}
    >
      全程只记分数 · 不涉及金钱
    </div>
  </Shell>
);

/* ───────── 04 房间同步（静态 UI 还原） ───────── */

const SEATS = [
  { name: "Alex星", initial: "A", score: 100, me: true },
  { name: "微信用户643", initial: "微", score: -100 },
  { name: "老王", initial: "王", score: -30 },
  { name: "阿铁", initial: "铁", score: 40 },
];

export const Card04Room = () => (
  <Shell page={4}>
    <Title
      lines={[{ text: "全桌实时同步，" }, { text: "笔笔清楚", accent: true }]}
      size={76}
      top={100}
    />
    {/* 手机 UI（静态） */}
    <div
      style={{
        position: "absolute",
        top: 330,
        left: 110,
        width: 860,
        borderRadius: 54,
        border: `2px solid ${T.onInkA28}`,
        background: T.ink900,
        boxShadow: `0 48px 90px rgba(0,0,0,0.6), 0 0 140px rgba(201,162,75,0.12)`,
        overflow: "hidden",
        paddingBottom: 36,
        boxSizing: "border-box",
      }}
    >
      {/* 状态栏 */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          padding: "24px 40px 0",
          fontSize: 26,
          fontWeight: 500,
        }}
      >
        <span>18:30</span>
        <span style={{ letterSpacing: 1 }}>5G ▮</span>
      </div>
      {/* nav */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "14px 36px 0",
        }}
      >
        <span style={{ color: T.brass, fontFamily: SERIF, fontSize: 40 }}>
          ‹
        </span>
        <div style={{ textAlign: "center" }}>
          <div
            style={{
              fontFamily: SERIF,
              fontWeight: 700,
              fontSize: 34,
              letterSpacing: "0.06em",
            }}
          >
            7avw 房间
          </div>
          <div style={{ fontSize: 20, color: T.dim, marginTop: 2 }}>
            4 人局 · 第 3 局
          </div>
        </div>
        <div style={{ width: 132 }} />
      </div>
      <ComplianceStrip />
      {/* 座位 */}
      <div
        style={{
          display: "flex",
          gap: 26,
          padding: "26px 36px 0",
          overflow: "hidden",
        }}
      >
        {SEATS.map((s) => (
          <div
            key={s.name}
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: 8,
            }}
          >
            <div
              style={{
                width: 78,
                height: 78,
                borderRadius: "50%",
                background: T.ink600,
                border: s.me ? `2.5px solid ${T.brass}` : "2.5px solid transparent",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontFamily: SERIF,
                fontWeight: 700,
                fontSize: 32,
                color: s.me ? T.brass : T.tileShade,
              }}
            >
              {s.initial}
            </div>
            <div style={{ fontSize: 19, color: T.dim }}>{s.name}</div>
            <Score value={s.score} style={{ fontSize: 32, fontWeight: 600 }} />
          </div>
        ))}
      </div>
      {/* 最近一手 */}
      <div
        style={{
          margin: "26px 36px 0",
          padding: "18px 24px",
          borderRadius: 18,
          background: T.tile,
          boxShadow: T.shadowTile,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          borderLeft: `5px solid ${T.brass}`,
        }}
      >
        <div>
          <div style={{ fontSize: 19, color: "rgba(42,39,34,0.5)", marginBottom: 4 }}>
            最近一手
          </div>
          <div style={{ fontSize: 26, color: T.jet, fontWeight: 500 }}>
            微信用户643 −100 → <b>Alex星 +100</b>
          </div>
        </div>
        <Score value={100} style={{ fontSize: 40, fontWeight: 600 }} />
      </div>
      {/* 流水一行 */}
      <div
        style={{
          margin: "18px 36px 0",
          padding: "20px 24px",
          borderRadius: 14,
          background: T.ink700,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <div>
          <div style={{ fontSize: 24 }}>老王 −30 → 阿铁 +30</div>
          <div style={{ fontSize: 18, color: T.dim, marginTop: 3 }}>
            09:27 · 房主确认
          </div>
        </div>
        <span style={{ fontSize: 22, color: T.dim, fontFamily: NUM }}>#2</span>
      </div>
      {/* 支出筹码 */}
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          gap: 10,
          marginTop: 30,
        }}
      >
        <div
          style={{
            width: 120,
            height: 120,
            borderRadius: "50%",
            background: `radial-gradient(circle at 32% 28%, ${T.brassHi}, ${T.brass} 60%, ${T.brassLo})`,
            boxShadow: T.shadowChip,
            border: `8px dashed rgba(20,18,15,0.35)`,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontFamily: SERIF,
            fontWeight: 700,
            fontSize: 34,
            color: T.jet,
          }}
        >
          支出
        </div>
        <span style={{ fontSize: 19, color: T.dim }}>记分入口 · 一键落账</span>
      </div>
    </div>
    <div
      style={{
        position: "absolute",
        top: 1318,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 30,
        letterSpacing: "0.1em",
        color: T.dim,
      }}
    >
      你记一笔，全桌立刻看到
    </div>
  </Shell>
);

/* ───────── 05 结算 ───────── */

const IOU_ROWS = [
  { from: "微信用户643", initial: "微", to: "Alex星", amount: 60 },
  { from: "老王", initial: "王", to: "Alex星", amount: 40 },
];

export const Card05Settle = () => (
  <Shell page={5}>
    <Title
      lines={[{ text: "散场不用吵，" }, { text: "一键结清", accent: true }]}
      size={84}
      top={100}
    />
    {/* 竖排战斗胜利 + 大牌 */}
    <div
      style={{
        position: "absolute",
        top: 360,
        left: 0,
        right: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        gap: 40,
      }}
    >
      <div
        style={{
          writingMode: "vertical-rl",
          fontFamily: SERIF,
          fontWeight: 700,
          fontSize: 66,
          letterSpacing: "0.34em",
          color: T.brass,
          textShadow: `0 4px 28px ${T.brassA50}`,
        }}
      >
        战斗胜利
      </div>
      <div style={{ position: "relative", width: 330, height: 430 }}>
        <div
          style={{
            position: "absolute",
            inset: 0,
            borderRadius: 20,
            background: T.tileEmboss,
            transform: "rotate(-8deg)",
          }}
        />
        <div
          style={{
            position: "absolute",
            inset: 0,
            borderRadius: 20,
            background: T.tileShade,
            transform: "rotate(6deg)",
          }}
        />
        <div
          style={{
            position: "absolute",
            inset: 0,
            borderRadius: 20,
            background: `linear-gradient(165deg, ${T.tile}, ${T.tileShade})`,
            border: `3px solid ${T.brass}`,
            transform: "rotate(-1.5deg)",
            boxShadow: "0 18px 40px rgba(0,0,0,0.55)",
            padding: "40px 24px 24px",
            boxSizing: "border-box",
            textAlign: "center",
          }}
        >
          <div
            style={{
              position: "absolute",
              top: 16,
              left: 22,
              fontFamily: SERIF,
              fontSize: 44,
              fontWeight: 700,
              color: "#B32D18",
            }}
          >
            A
          </div>
          <div
            style={{
              fontSize: 20,
              letterSpacing: "0.24em",
              color: "rgba(42,39,34,0.55)",
            }}
          >
            总 计
          </div>
          <div
            style={{
              fontFamily: NUM,
              fontSize: 100,
              fontWeight: 600,
              color: T.brassLo,
              lineHeight: 1.1,
              fontVariantNumeric: "tabular-nums",
            }}
          >
            100
          </div>
          <div
            style={{
              fontFamily: SERIF,
              fontWeight: 700,
              fontSize: 26,
              color: T.jet,
            }}
          >
            Alex星
          </div>
          <div
            style={{
              position: "absolute",
              right: 18,
              bottom: 12,
              fontSize: 56,
              color: "#B32D18",
            }}
          >
            ♥
          </div>
        </div>
      </div>
    </div>
    {/* IOU 行 */}
    <div
      style={{
        position: "absolute",
        top: 870,
        left: 90,
        right: 90,
        display: "flex",
        flexDirection: "column",
        gap: 22,
      }}
    >
      {IOU_ROWS.map((r) => (
        <TileCard
          key={r.from}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 20,
            padding: "26px 30px",
          }}
        >
          <div
            style={{
              width: 60,
              height: 60,
              borderRadius: "50%",
              background: T.ink600,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontFamily: SERIF,
              fontWeight: 700,
              fontSize: 26,
              color: T.tileShade,
              flex: "none",
            }}
          >
            {r.initial}
          </div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 30, fontWeight: 500, color: T.jet }}>
              {r.from} → {r.to}
            </div>
            <div style={{ fontSize: 23, color: "rgba(42,39,34,0.55)", marginTop: 4 }}>
              记给 {r.to} · 线下自行处理
            </div>
          </div>
          <Score
            value={r.amount}
            signed={false}
            style={{ fontSize: 46, fontWeight: 600, color: T.lose }}
          />
        </TileCard>
      ))}
    </div>
    <div
      style={{
        position: "absolute",
        top: 1240,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 32,
        letterSpacing: "0.1em",
        color: T.dim,
      }}
    >
      谁记给谁多少，一眼看清 · 策略参考，可不按
    </div>
  </Shell>
);

/* ───────── 06 战绩 ───────── */

const MONTHS = [
  { m: "7月", v: 320 },
  { m: "8月", v: 180 },
  { m: "9月", v: 520 },
  { m: "10月", v: 420 },
];
const MAX_V = 520;

export const Card06Stats = () => (
  <Shell page={6}>
    <Title
      lines={[{ text: "每一局，" }, { text: "都算得清", accent: true }]}
      size={96}
      top={130}
    />
    {/* 三格统计 */}
    <div
      style={{
        position: "absolute",
        top: 420,
        left: 0,
        right: 0,
        display: "flex",
        gap: 26,
        justifyContent: "center",
      }}
    >
      {[
        { v: "32", l: "总局数" },
        { v: "58%", l: "胜率" },
        { v: "+1,240", l: "累计净胜", a: true },
      ].map((s) => (
        <TileCard
          key={s.l}
          style={{ width: 280, padding: "34px 0 30px", textAlign: "center" }}
        >
          <div
            style={{
              fontFamily: NUM,
              fontSize: 64,
              fontWeight: 600,
              color: s.a ? T.brassLo : T.jet,
              fontVariantNumeric: "tabular-nums",
            }}
          >
            {s.v}
          </div>
          <div
            style={{
              fontSize: 24,
              color: "rgba(42,39,34,0.55)",
              marginTop: 8,
              letterSpacing: "0.1em",
            }}
          >
            {s.l}
          </div>
        </TileCard>
      ))}
    </div>
    {/* 月度条 */}
    <div style={{ position: "absolute", top: 700, left: 90, right: 90 }}>
      <div
        style={{
          display: "flex",
          alignItems: "baseline",
          justifyContent: "space-between",
          marginBottom: 44,
        }}
      >
        <span
          style={{
            fontFamily: SERIF,
            fontSize: 34,
            fontWeight: 700,
            letterSpacing: "0.18em",
          }}
        >
          2026年10月 · 净胜
        </span>
        <span
          style={{
            fontFamily: NUM,
            fontSize: 120,
            fontWeight: 600,
            color: T.brass,
            lineHeight: 1,
            textShadow: `0 4px 28px ${T.brassA50}`,
            fontVariantNumeric: "tabular-nums",
          }}
        >
          +420
        </span>
      </div>
      {MONTHS.map((mo) => (
        <div
          key={mo.m}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 24,
            marginBottom: 30,
          }}
        >
          <span style={{ width: 70, fontSize: 27, color: T.dim }}>{mo.m}</span>
          <div
            style={{
              flex: 1,
              height: 30,
              borderRadius: 999,
              background: T.ink700,
              overflow: "hidden",
            }}
          >
            <div
              style={{
                width: `${(mo.v / MAX_V) * 100}%`,
                height: "100%",
                borderRadius: 999,
                background: `linear-gradient(90deg, ${T.brassLo}, ${T.brassHi})`,
                boxShadow: `0 0 14px ${T.brassA50}`,
              }}
            />
          </div>
          <Score value={mo.v} style={{ width: 110, textAlign: "right", fontSize: 34 }} />
        </div>
      ))}
    </div>
    <div
      style={{
        position: "absolute",
        top: 1260,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 30,
        letterSpacing: "0.1em",
        color: T.dim,
      }}
    >
      战绩随时回看 · 按月统计，胜负明明白白
    </div>
  </Shell>
);

/* ───────── 07 合规声明 ───────── */

export const Card07Compliance = () => (
  <Shell page={7}>
    <Title
      lines={[{ text: "先说清楚" }, { text: "合规这件事", accent: true }]}
      size={96}
      top={150}
    />
    <div
      style={{
        position: "absolute",
        top: 440,
        left: 100,
        right: 100,
        padding: "56px 60px",
        borderRadius: 32,
        background: T.complyBg,
        border: `2px solid ${T.complyBorder}`,
        textAlign: "center",
        boxShadow: "0 24px 60px rgba(0,0,0,0.4)",
      }}
    >
      <div
        style={{
          fontFamily: SERIF,
          fontWeight: 700,
          fontSize: 44,
          letterSpacing: "0.3em",
          color: T.complyStrong,
        }}
      >
        合规声明
      </div>
      <div
        style={{
          margin: "34px 0",
          padding: "34px 0",
          borderTop: `2px dashed ${T.complyBorder}`,
          borderBottom: `2px dashed ${T.complyBorder}`,
          fontSize: 36,
          lineHeight: 2,
          letterSpacing: "0.04em",
        }}
      >
        本小程序仅为记分工具
        <br />
        不涉及金钱交易 · 禁止用于赌博
        <br />
        分数仅作娱乐记录
      </div>
      <div
        style={{
          fontFamily: SERIF,
          fontWeight: 700,
          fontSize: 56,
          letterSpacing: "0.14em",
          color: T.brass,
          textShadow: `0 4px 24px ${T.brassA50}`,
        }}
      >
        只记分数，不涉及金钱
      </div>
    </div>
    <div
      style={{
        position: "absolute",
        top: 1140,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 34,
        letterSpacing: "0.12em",
        color: T.dim,
      }}
    >
      娱乐记分 · 友谊第一
    </div>
  </Shell>
);

/* ───────── 08 尾页 ───────── */

export const Card08End = () => (
  <Shell page={8}>
    <div
      style={{
        position: "absolute",
        top: 300,
        left: 0,
        right: 0,
        textAlign: "center",
      }}
    >
      <div
        style={{
          fontFamily: SERIF,
          fontWeight: 700,
          fontSize: 130,
          letterSpacing: "0.16em",
          color: T.brass,
          textShadow: `0 6px 36px ${T.brassA50}`,
        }}
      >
        记账局叙
      </div>
      <div
        style={{
          marginTop: 34,
          fontFamily: SERIF,
          fontWeight: 500,
          fontSize: 48,
          letterSpacing: "0.28em",
        }}
      >
        散场不散账，一键算清
      </div>
    </div>
    {/* 三步回顾 */}
    <div
      style={{
        position: "absolute",
        top: 640,
        left: 120,
        right: 120,
        display: "flex",
        justifyContent: "space-between",
        gap: 20,
      }}
    >
      {["开一局", "随手记", "一键结清"].map((t, i) => (
        <div
          key={t}
          style={{
            flex: 1,
            padding: "30px 0",
            borderRadius: 20,
            background: T.ink700,
            border: `1.5px solid ${T.onInkA28}`,
            textAlign: "center",
            fontFamily: SERIF,
            fontWeight: 700,
            fontSize: 34,
            letterSpacing: "0.08em",
          }}
        >
          <div
            style={{
              fontFamily: NUM,
              fontSize: 40,
              color: T.brass,
              marginBottom: 8,
            }}
          >
            {i + 1}
          </div>
          {t}
        </div>
      ))}
    </div>
    {/* 收藏 CTA */}
    <div
      style={{
        position: "absolute",
        top: 960,
        left: 0,
        right: 0,
        display: "flex",
        justifyContent: "center",
      }}
    >
      <div
        style={{
          padding: "34px 76px",
          borderRadius: 999,
          background: `linear-gradient(180deg, ${T.brassHi}, ${T.brass})`,
          boxShadow: `0 6px 0 ${T.brassLo}, 0 18px 40px rgba(0,0,0,0.45)`,
          fontFamily: SERIF,
          fontWeight: 700,
          fontSize: 44,
          letterSpacing: "0.16em",
          color: T.jet,
        }}
      >
        ♥ 收藏这条笔记 · 开局随手记
      </div>
    </div>
    <div
      style={{
        position: "absolute",
        top: 1210,
        left: 0,
        right: 0,
        textAlign: "center",
        fontSize: 30,
        letterSpacing: "0.1em",
        color: T.dim,
      }}
    >
      免费记分工具 · 只记分数，不涉及金钱
    </div>
  </Shell>
);
