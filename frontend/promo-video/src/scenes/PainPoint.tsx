import { interpolate, useCurrentFrame } from "remotion";
import { SERIF, SANS, T } from "../theme";
import { Bg, Headline, useEnter, Voice, type VoiceCfg } from "../ui";

/** 散落的牌（静态残局） */
const CARDS: {
  x: number;
  y: number;
  rot: number;
  rank: string;
  suit: string;
  red: boolean;
  blur?: number;
}[] = [
  { x: 90, y: 1490, rot: -14, rank: "K", suit: "♠", red: false, blur: 1.5 },
  { x: 300, y: 1560, rot: 8, rank: "7", suit: "♥", red: true },
  { x: 520, y: 1500, rot: -4, rank: "A", suit: "♣", red: false },
  { x: 730, y: 1580, rot: 16, rank: "Q", suit: "♦", red: true, blur: 1 },
  { x: 920, y: 1470, rot: -10, rank: "10", suit: "♥", red: true, blur: 2 },
];

const ScatterCard: React.FC<(typeof CARDS)[number]> = ({
  x,
  y,
  rot,
  rank,
  suit,
  red,
  blur,
}) => {
  const p = useEnter(Math.round((x / 1080) * 14) + 4, { stiffness: 70 });
  return (
    <div
      style={{
        position: "absolute",
        left: x,
        top: y,
        width: 168,
        height: 232,
        borderRadius: 16,
        background: `linear-gradient(160deg, ${T.tile}, ${T.tileShade})`,
        boxShadow: T.shadowTile,
        rotate: `${rot}deg`,
        translate: `0px ${(1 - p) * -90}px`,
        opacity: p,
        filter: blur ? `blur(${blur}px)` : undefined,
        padding: "14px 16px",
        boxSizing: "border-box",
        fontFamily: SERIF,
        fontWeight: 700,
        color: red ? "#B32D18" : T.jet,
      }}
    >
      <div style={{ fontSize: 34, lineHeight: 1 }}>{rank}</div>
      <div style={{ fontSize: 30, lineHeight: 1.2 }}>{suit}</div>
      <div
        style={{
          position: "absolute",
          right: 16,
          bottom: 12,
          fontSize: 64,
          opacity: 0.9,
        }}
      >
        {suit}
      </div>
    </div>
  );
};

/** 第一幕：散场残局 + 痛点大字 */
export const PainPoint: React.FC<{
  /** 痛点问句（小红书版用「谁输谁赢？」去金钱联想） */
  question?: string;
  vo?: VoiceCfg;
}> = ({ question = "谁欠谁多少？", vo }) => {
  const frame = useCurrentFrame();
  const sub = interpolate(frame, [70, 90], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const v = vo ?? { src: "voice/s1.wav", start: 0, dur: 115, rate: 1.25 };

  return (
    <Bg>
      {/* 旁白 */}
      <Voice cfg={v} />
      {/* 牌桌桌布（下半弧面） */}
      <div
        style={{
          position: "absolute",
          left: "-12%",
          right: "-12%",
          bottom: -320,
          height: 1060,
          borderRadius: "50%",
          background: `radial-gradient(ellipse at 50% 28%, ${T.feltHi}, ${T.felt} 55%, ${T.feltDeep} 100%)`,
          boxShadow: "inset 0 50px 100px rgba(0,0,0,0.55)",
        }}
      />
      <div
        style={{
          position: "absolute",
          left: "-12%",
          right: "-12%",
          bottom: -320,
          height: 1060,
          borderRadius: "50%",
          border: `5px solid ${T.brassLo}`,
          opacity: 0.45,
        }}
      />
      {/* 桌心聚光 */}
      <div
        style={{
          position: "absolute",
          left: "50%",
          top: 1180,
          width: 700,
          height: 360,
          translate: "-50% 0",
          background:
            "radial-gradient(ellipse, rgba(201,162,75,0.14), transparent 70%)",
        }}
      />
      {CARDS.map((c) => (
        <ScatterCard key={`${c.x}`} {...c} />
      ))}
      {/* 筹码 */}
      <div
        style={{
          position: "absolute",
          left: 470,
          top: 1375,
          width: 96,
          height: 96,
          borderRadius: "50%",
          background: `radial-gradient(circle at 35% 30%, ${T.brassHi}, ${T.brassLo})`,
          boxShadow: T.shadowChip,
          border: `6px dashed ${T.ink900}`,
          opacity: useEnter(10),
        }}
      />
      <div
        style={{
          position: "absolute",
          left: 545,
          top: 1415,
          width: 96,
          height: 96,
          borderRadius: "50%",
          background: `radial-gradient(circle at 35% 30%, #6E675C, #3A362F)`,
          boxShadow: T.shadowChip,
          border: `6px dashed ${T.ink900}`,
          rotate: "12deg",
          opacity: useEnter(16),
        }}
      />

      <Headline
        lines={[{ text: "散场了" }, { text: question, accent: true }]}
        delay={12}
        size={110}
        top={430}
      />
      <div
        style={{
          position: "absolute",
          top: 790,
          left: 0,
          right: 0,
          textAlign: "center",
          fontFamily: SANS,
          fontSize: 44,
          letterSpacing: "0.12em",
          color: T.dim,
          opacity: sub,
          translate: `0px ${(1 - sub) * 24}px`,
        }}
      >
        口头难算清，心里更没底
      </div>
    </Bg>
  );
};
