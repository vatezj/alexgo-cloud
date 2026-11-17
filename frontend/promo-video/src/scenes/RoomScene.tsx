import { interpolate, useCurrentFrame } from "remotion";
import { SERIF, T } from "../theme";
import { Bg, ComplianceStrip, Headline, PhoneShell, Score, useEnter, Voice, type VoiceCfg } from "../ui";

const SEATS = [
  { name: "Alex星", initial: "A", score: 100, me: true },
  { name: "微信用户643", initial: "微", score: -100 },
  { name: "老王", initial: "王", score: -30 },
  { name: "阿铁", initial: "铁", score: 40 },
];

const LEDGER = [
  { text: "微信用户643 −100 → Alex星 +100", time: "09:41", score: 100 },
  { text: "老王 −30 → 阿铁 +30", time: "09:27", score: 30 },
  { text: "阿铁 −60 → 微信用户643 +60", time: "09:18", score: 60 },
];

type SeatData = (typeof SEATS)[number];

/** 单座位（筹码轨） */
const Seat: React.FC<{ s: SeatData; delay: number }> = ({ s, delay }) => {
  const p = useEnter(delay);
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: 8,
        opacity: p,
        translate: `0px ${(1 - p) * 24}px`,
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
          boxShadow: s.me ? `0 0 18px ${T.brassA50}` : undefined,
        }}
      >
        {s.initial}
      </div>
      <div
        style={{
          fontSize: 19,
          color: T.dim,
          maxWidth: 120,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {s.name}
      </div>
      <Score value={s.score} style={{ fontSize: 32, fontWeight: 600 }} />
    </div>
  );
};

/** 流水行 */
const LedgerRow: React.FC<{
  row: (typeof LEDGER)[number];
  delay: number;
  rank: number;
  highlight?: boolean;
}> = ({ row, delay, rank, highlight }) => {
  const p = useEnter(delay);
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        padding: "20px 24px",
        marginBottom: 12,
        borderRadius: 14,
        background: highlight ? T.ink700 : T.ink700,
        opacity: p * (highlight ? 1 : 0.85),
        translate: `0px ${(1 - p) * 24}px`,
      }}
    >
      <div>
        <div style={{ fontSize: 24, color: T.onInk }}>{row.text}</div>
        <div style={{ fontSize: 18, color: T.dim, marginTop: 3 }}>
          {row.time} · 房主确认
        </div>
      </div>
      <span
        style={{
          fontSize: 22,
          color: T.dim,
          fontFamily: '"Oswald", sans-serif',
        }}
      >
        #{rank}
      </span>
    </div>
  );
};

/** 第三幕：房间俯视牌桌 + 记一笔落账 */
export const RoomScene: React.FC<{ vo?: VoiceCfg }> = ({ vo }) => {
  const frame = useCurrentFrame();
  // 支出筹码涟漪：f30 触发
  const ripple = interpolate(frame, [30, 60], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  // 新一手落账：f45 滑入
  const newHand = useEnter(45, { stiffness: 130 });
  const chipP = useEnter(36, { stiffness: 160, damping: 12 });
  const v = vo ?? { src: "voice/s3.wav", start: 12, dur: 137, rate: 1.05 };

  return (
    <Bg>
      {/* 旁白：记一笔 */}
      <Voice cfg={v} />
      <Headline
        lines={[{ text: "记一笔" }, { text: "牌桌同步到账", accent: true }]}
        delay={10}
        size={92}
        top={130}
      />
      <div
        style={{
          position: "absolute",
          top: 430,
          left: 0,
          right: 0,
          display: "flex",
          justifyContent: "center",
        }}
      >
        <PhoneShell width={860} delay={20}>
          {/* nav */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              padding: "14px 36px 0",
            }}
          >
            <span
              style={{
                color: T.brass,
                fontFamily: SERIF,
                fontSize: 40,
                lineHeight: 1,
              }}
            >
              ‹
            </span>
            <div style={{ textAlign: "center" }}>
              <div
                style={{
                  fontFamily: SERIF,
                  fontWeight: 700,
                  fontSize: 34,
                  letterSpacing: "0.06em",
                  color: T.onInk,
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

          {/* 座位筹码轨 */}
          <div
            style={{
              display: "flex",
              gap: 26,
              padding: "26px 36px 0",
              overflow: "hidden",
            }}
          >
            {SEATS.map((s, i) => (
              <Seat key={s.name} s={s} delay={26 + i * 6} />
            ))}
            {/* 加位虚座 */}
            <div
              style={{
                width: 78,
                height: 78,
                borderRadius: "50%",
                border: `2px dashed ${T.onInkA28}`,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color: T.dim,
                fontSize: 36,
                flex: "none",
                marginTop: 0,
              }}
            >
              +
            </div>
          </div>

          {/* 中央最近一手 */}
          <div
            style={{
              margin: "26px 36px 0",
              borderRadius: 18,
              background: T.tile,
              boxShadow: T.shadowTile,
              overflow: "hidden",
              position: "relative",
              height: 148,
            }}
          >
            <div
              style={{
                padding: "16px 24px 0",
                fontSize: 19,
                letterSpacing: "0.16em",
                color: "rgba(42,39,34,0.5)",
              }}
            >
              最近一手
            </div>
            {/* 旧一手（下移让位） */}
            <div
              style={{
                position: "absolute",
                left: 0,
                right: 0,
                top: 52,
                padding: "0 24px",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                translate: `0px ${newHand * 64}px`,
                opacity: 1 - newHand,
              }}
            >
              <span style={{ fontSize: 26, color: T.jet, fontWeight: 500 }}>
                老王 −30 → 阿铁 +30
              </span>
              <Score value={30} style={{ fontSize: 36 }} />
            </div>
            {/* 新一手（滑入高亮） */}
            <div
              style={{
                position: "absolute",
                left: 0,
                right: 0,
                top: 52,
                padding: "0 24px",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                borderLeft: `5px solid ${T.brass}`,
                background:
                  "linear-gradient(90deg, rgba(201,162,75,0.18), transparent 65%)",
                translate: `0px ${(1 - newHand) * -100}px`,
                opacity: newHand,
              }}
            >
              <span style={{ fontSize: 26, color: T.jet, fontWeight: 500 }}>
                微信用户643 −100 → <b>Alex星 +100</b>
              </span>
              <Score value={100} style={{ fontSize: 40, fontWeight: 600 }} />
            </div>
          </div>

          {/* 本局流水 */}
          <div style={{ margin: "26px 36px 0" }}>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "baseline",
                marginBottom: 14,
              }}
            >
              <span
                style={{
                  fontFamily: SERIF,
                  fontSize: 26,
                  fontWeight: 500,
                  letterSpacing: "0.2em",
                  color: T.brass,
                }}
              >
                本局流水
              </span>
              <span style={{ fontSize: 20, color: T.dim }}>
                09:44 已同步 · 全员可见
              </span>
            </div>
            {LEDGER.map((r, i) => (
              <LedgerRow
                key={r.time}
                row={r}
                delay={30 + i * 8}
                rank={LEDGER.length - i}
                highlight={i === 0}
              />
            ))}
          </div>

          {/* 支出筹码 */}
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: 10,
              padding: "20px 0 36px",
            }}
          >
            <div style={{ position: "relative", width: 130, height: 130 }}>
              {/* 涟漪 */}
              <div
                style={{
                  position: "absolute",
                  inset: 0,
                  borderRadius: "50%",
                  border: `3px solid ${T.brass}`,
                  scale: `${1 + ripple * 0.9}`,
                  opacity: ripple > 0 ? (1 - ripple) * 0.7 : 0,
                }}
              />
              <div
                style={{
                  position: "absolute",
                  inset: 0,
                  borderRadius: "50%",
                  background: `radial-gradient(circle at 32% 28%, ${T.brassHi}, ${T.brass} 60%, ${T.brassLo})`,
                  boxShadow: T.shadowChip,
                  border: `8px dashed rgba(20,18,15,0.35)`,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontFamily: SERIF,
                  fontWeight: 700,
                  fontSize: 36,
                  letterSpacing: "0.1em",
                  color: T.jet,
                  scale: `${(0.9 + 0.1 * chipP) * (1 - ripple * 0.06)}`,
                  translate: `0px ${(1 - chipP) * 40}px`,
                }}
              >
                支出
              </div>
            </div>
            <span style={{ fontSize: 19, color: T.dim, letterSpacing: "0.08em" }}>
              记分入口 · 一键落账
            </span>
          </div>
        </PhoneShell>
      </div>
    </Bg>
  );
};
