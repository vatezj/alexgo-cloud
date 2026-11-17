import { Easing, interpolate, useCurrentFrame } from "remotion";
import { SERIF, T } from "../theme";
import { Bg, Capsule, Count, Headline, Score, useEnter, Voice, type VoiceCfg } from "../ui";

type IouRowData = {
  from: string;
  fromInitial: string;
  to: string;
  amount: number;
};

const IOU_ROWS: IouRowData[] = [
  { from: "微信用户643", fromInitial: "微", to: "Alex星", amount: 60 },
  { from: "老王", fromInitial: "王", to: "Alex星", amount: 40 },
];

/** IOU 行（贪心简化展示） */
const IouRow: React.FC<{ row: IouRowData; delay: number; idx: number }> = ({
  row,
  delay,
  idx,
}) => {
  const frame = useCurrentFrame();
  const p = interpolate(frame, [delay, delay + 20], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.bezier(0.16, 1, 0.3, 1),
  });
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 20,
        marginBottom: 16,
        padding: "24px 26px",
        borderRadius: 18,
        background: T.tile,
        boxShadow: T.shadowTile,
        opacity: p,
        translate: `${(1 - p) * -70}px 0px`,
      }}
    >
      <div
        style={{
          width: 64,
          height: 64,
          borderRadius: "50%",
          background: T.ink600,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontFamily: SERIF,
          fontWeight: 700,
          fontSize: 28,
          color: T.tileShade,
          flex: "none",
        }}
      >
        {row.fromInitial}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 28, fontWeight: 500, color: T.jet }}>
          {row.from} → {row.to}
        </div>
        <div style={{ fontSize: 21, color: "rgba(42,39,34,0.55)", marginTop: 4 }}>
          记给 {row.to} · 线下自行处理 · 第 {idx + 1} 笔
        </div>
      </div>
      {/* 与结算页 calcIou 展示一致：金额走输色（tone=lose） */}
      <Score
        value={row.amount}
        signed={false}
        style={{ fontSize: 46, fontWeight: 600, color: T.lose }}
      />
    </div>
  );
};

/** 第四幕（高潮）：结算页 · 战斗胜利 + IOU 一键看清 */
export const SettleScene: React.FC<{ vo?: VoiceCfg }> = ({ vo }) => {
  const frame = useCurrentFrame();
  const v = vo ?? { src: "voice/s4.wav", start: 20, dur: 173 };

  // 竖排「战斗胜利」自上而下揭开
  const titleClip = interpolate(frame, [12, 48], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.bezier(0.16, 1, 0.3, 1),
  });
  const heroP = useEnter(16, { stiffness: 100 });
  // 状态翻转：已结算（f215）
  const settled = frame >= 215;
  // 按钮按压（f195–215）
  const press = interpolate(frame, [195, 203, 215], [1, 0.965, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  // Toast「已结算」
  const toastP = interpolate(frame, [222, 236, 258, 266], [0, 1, 1, 0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.bezier(0.16, 1, 0.3, 1),
  });

  return (
    <Bg>
      {/* 旁白：一键结清 */}
      <Voice cfg={v} />
      {/* nav */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "30px 48px 0",
        }}
      >
        <span style={{ color: T.brass, fontFamily: SERIF, fontSize: 44 }}>
          ‹
        </span>
        <div style={{ textAlign: "center" }}>
          <div
            style={{
              fontFamily: SERIF,
              fontWeight: 700,
              fontSize: 38,
              letterSpacing: "0.08em",
              color: T.onInk,
            }}
          >
            结算
          </div>
          <div style={{ fontSize: 21, color: T.dim, marginTop: 2 }}>
            7avw 房间 · 第 3 局
          </div>
        </div>
        <Capsule />
      </div>

      {/* 顶部字幕区（y≈130–320） */}
      <Headline
        lines={[{ text: "一键结清" }]}
        delay={30}
        size={86}
        top={130}
        exit={105}
      />
      <Headline
        lines={[{ text: "谁给谁多少，一眼看清", accent: true }]}
        delay={120}
        size={76}
        top={150}
        exit={190}
      />
      {/* Toast */}
      <div
        style={{
          position: "absolute",
          top: 150,
          left: 0,
          right: 0,
          display: "flex",
          justifyContent: "center",
          opacity: toastP,
          translate: `0px ${(1 - toastP) * -30}px`,
          pointerEvents: "none",
        }}
      >
        <div
          style={{
            padding: "16px 44px",
            borderRadius: 999,
            background: T.ink700,
            border: `1px solid ${T.brassA50}`,
            fontFamily: SERIF,
            fontWeight: 700,
            fontSize: 34,
            letterSpacing: "0.14em",
            color: T.brass,
            boxShadow: "0 12px 30px rgba(0,0,0,0.45)",
          }}
        >
          ✓ 已结算
        </div>
      </div>

      {/* 排行大牌 */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 44,
          marginTop: 210,
        }}
      >
        {/* 竖排战斗胜利 */}
        <div
          style={{
            writingMode: "vertical-rl",
            fontFamily: SERIF,
            fontWeight: 700,
            fontSize: 84,
            letterSpacing: "0.34em",
            color: T.brass,
            textShadow: `0 4px 28px ${T.brassA50}`,
            clipPath: `inset(0 0 ${(1 - titleClip) * 100}% 0)`,
            lineHeight: 1.1,
          }}
        >
          战斗胜利
        </div>
        {/* 卡牌堆 */}
        <div
          style={{
            position: "relative",
            width: 430,
            height: 550,
            opacity: heroP,
            scale: `${0.85 + 0.15 * heroP}`,
            translate: `0px ${(1 - heroP) * 50}px`,
          }}
        >
          <div
            style={{
              position: "absolute",
              inset: 0,
              borderRadius: 22,
              background: T.tileEmboss,
              transform: "rotate(-8deg)",
              boxShadow: "0 10px 24px rgba(0,0,0,0.5)",
            }}
          />
          <div
            style={{
              position: "absolute",
              inset: 0,
              borderRadius: 22,
              background: T.tileShade,
              transform: "rotate(6deg)",
              boxShadow: "0 10px 24px rgba(0,0,0,0.5)",
            }}
          />
          <div
            style={{
              position: "absolute",
              inset: 0,
              borderRadius: 22,
              background: `linear-gradient(165deg, ${T.tile}, ${T.tileShade})`,
              border: `3px solid ${T.brass}`,
              transform: "rotate(-1.5deg)",
              boxShadow: "0 18px 40px rgba(0,0,0,0.55)",
              padding: "54px 30px 30px",
              boxSizing: "border-box",
              textAlign: "center",
            }}
          >
            <div
              style={{
                fontFamily: SERIF,
                fontSize: 56,
                fontWeight: 700,
                color: "#B32D18",
                position: "absolute",
                top: 22,
                left: 28,
                lineHeight: 1,
              }}
            >
              A
            </div>
            <div
              style={{
                fontSize: 24,
                letterSpacing: "0.24em",
                color: "rgba(42,39,34,0.55)",
              }}
            >
              总 计
            </div>
            <Count
              from={0}
              to={100}
              start={22}
              end={72}
              style={{
                display: "block",
                marginTop: 8,
                fontSize: 128,
                fontWeight: 600,
                color: T.brassLo,
                lineHeight: 1.05,
              }}
            />
            <div
              style={{
                marginTop: 20,
                fontFamily: SERIF,
                fontWeight: 700,
                fontSize: 32,
                letterSpacing: "0.06em",
                color: T.jet,
              }}
            >
              Alex星
            </div>
            <div
              style={{
                position: "absolute",
                right: 24,
                bottom: 18,
                fontSize: 72,
                color: "#B32D18",
                lineHeight: 1,
              }}
            >
              ♥
            </div>
          </div>
        </div>
      </div>

      {/* IOU 区 */}
      <div style={{ margin: "34px 90px 0" }}>
        <div
          style={{
            textAlign: "center",
            fontFamily: SERIF,
            fontWeight: 700,
            fontSize: 32,
            letterSpacing: "0.2em",
            color: settled ? T.jade : T.onInk,
            marginBottom: 18,
          }}
        >
          {settled ? "已结算" : "未结算"} ·应收应付
        </div>
        {IOU_ROWS.map((r, i) => (
          <IouRow key={r.from} row={r} delay={78 + i * 24} idx={i} />
        ))}
        <div
          style={{
            textAlign: "center",
            fontSize: 23,
            color: T.dim,
            letterSpacing: "0.08em",
            marginTop: 4,
          }}
        >
          策略参考，可不按 · 只记分数，不涉及金钱
        </div>
      </div>

      {/* 操作区 */}
      <div
        style={{
          position: "absolute",
          left: 0,
          right: 0,
          bottom: 70,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          gap: 16,
        }}
      >
        <div
          style={{
            width: 720,
            padding: "30px 0",
            borderRadius: 18,
            background: settled
              ? T.onInkA04
              : `linear-gradient(180deg, ${T.brassHi}, ${T.brass})`,
            border: settled
              ? `1.5px solid ${T.onInkA28}`
              : `1.5px solid ${T.brass}`,
            boxShadow: settled
              ? "none"
              : `0 6px 0 ${T.brassLo}, 0 18px 34px rgba(0,0,0,0.45)`,
            fontFamily: SERIF,
            fontWeight: 700,
            fontSize: 40,
            letterSpacing: "0.24em",
            color: settled ? T.dim : T.jet,
            textAlign: "center",
            scale: `${press}`,
          }}
        >
          {settled ? "本局已封账" : "结束牌局"}
        </div>
        <div style={{ fontSize: 22, color: T.dim, letterSpacing: "0.1em" }}>
          只记分数，不涉及金钱 · 线下自行处理
        </div>
      </div>

      {/* 底部字幕叠层安全位（结算页自带操作区，不另叠） */}
    </Bg>
  );
};
