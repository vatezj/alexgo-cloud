import { Audio } from "@remotion/media";
import type { CSSProperties, ReactNode } from "react";
import {
  AbsoluteFill,
  Easing,
  interpolate,
  Sequence,
  spring,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { NUM, SANS, T } from "./theme";

/** 旁白配置：src 音频、start 场景内起始帧、dur 播放帧数、rate 倍速 */
export type VoiceCfg = {
  src: string;
  start: number;
  dur: number;
  rate?: number;
};

/** 按 cfg 输出旁白 Audio（淡入 5 帧、尾部 9 帧淡出） */
export const Voice: React.FC<{ cfg: VoiceCfg }> = ({ cfg }) => (
  <Sequence from={cfg.start}>
    <Audio
      src={staticFile(cfg.src)}
      playbackRate={cfg.rate ?? 1}
      volume={(f) =>
        interpolate(f, [0, 5, cfg.dur - 9, cfg.dur], [0, 1, 1, 0], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
        })
      }
    />
  </Sequence>
);

/** 全片底：墨面 + 顶部黄铜聚光 */
export const Bg: React.FC<{ children?: ReactNode }> = ({ children }) => (
  <AbsoluteFill
    style={{
      backgroundColor: T.ink900,
      backgroundImage: T.spot,
      fontFamily: SANS,
      color: T.onInk,
      overflow: "hidden",
    }}
  >
    {children}
  </AbsoluteFill>
);

/** 入场进度：delay 帧后 0→1 弹簧 */
export const useEnter = (
  delay: number,
  config: { damping?: number; stiffness?: number } = {},
): number => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  return spring({
    frame: frame - delay,
    fps,
    config: { damping: 200, stiffness: 100, ...config },
  });
};

/** 淡入上移进度（缓出） */
export const useFadeUp = (delay: number, dur = 18): number => {
  const frame = useCurrentFrame();
  return interpolate(frame, [delay, delay + dur], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.bezier(0.16, 1, 0.3, 1),
  });
};

type HeadlineProps = {
  lines: { text: string; accent?: boolean }[];
  delay?: number;
  size?: number;
  top?: number;
  /** 淡出起点（不传则常驻） */
  exit?: number;
};

/** 大字幕（主标 ≥84px，安全区左右 ≥80） */
export const Headline: React.FC<HeadlineProps> = ({
  lines,
  delay = 0,
  size = 88,
  top = 150,
  exit,
}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const exitP =
    exit === undefined
      ? 1
      : interpolate(frame, [exit, exit + 12], [1, 0], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
        });
  return (
    <div
      style={{
        position: "absolute",
        top,
        left: 0,
        right: 0,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: 6,
        opacity: exitP,
        pointerEvents: "none",
      }}
    >
      {lines.map((ln, i) => {
        const t0 = delay + i * Math.round(fps * 0.3);
        const p = interpolate(frame, [t0, t0 + Math.round(fps * 0.6)], [0, 1], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
          easing: Easing.bezier(0.16, 1, 0.3, 1),
        });
        return (
          <div
            key={ln.text}
            style={{
              opacity: p,
              translate: `0px ${(1 - p) * 44}px`,
              fontFamily: '"Noto Serif SC", serif',
              fontWeight: 700,
              fontSize: size,
              lineHeight: 1.3,
              letterSpacing: "0.08em",
              color: ln.accent ? T.brass : T.onInk,
              textShadow: ln.accent ? `0 4px 24px ${T.brassA50}` : "none",
              textAlign: "center",
            }}
          >
            {ln.text}
          </div>
        );
      })}
    </div>
  );
};

/** 小程序顶部胶囊（微信真机装饰位） */
export const Capsule: React.FC<{ scale?: number }> = ({ scale = 1 }) => (
  <div
    style={{
      display: "flex",
      width: 132 * scale,
      height: 48 * scale,
      alignItems: "center",
      border: `1.5px solid ${T.onInkA28}`,
      borderRadius: 999,
      background: T.onInkA04,
      flex: "none",
    }}
  >
    <div
      style={{
        flex: 1,
        color: T.onInk,
        fontSize: 20 * scale,
        letterSpacing: 2,
        textAlign: "center",
        lineHeight: 1,
      }}
    >
      •••
    </div>
    <div
      style={{
        width: 1.5,
        height: 26 * scale,
        background: T.onInkA28,
      }}
    />
    <div style={{ flex: 1, display: "flex", justifyContent: "center" }}>
      <div
        style={{
          width: 20 * scale,
          height: 20 * scale,
          border: `3px solid ${T.onInk}`,
          borderRadius: "50%",
          position: "relative",
        }}
      >
        <div
          style={{
            position: "absolute",
            inset: 4 * scale,
            borderRadius: "50%",
            background: T.onInk,
          }}
        />
      </div>
    </div>
  </div>
);

/** 状态栏（时间 + 信号/电池） */
export const StatusBar: React.FC = () => (
  <div
    style={{
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center",
      padding: "24px 40px 0",
      fontSize: 26,
      fontWeight: 500,
      color: T.onInk,
    }}
  >
    <span>18:30</span>
    <span style={{ display: "flex", alignItems: "center", gap: 10 }}>
      <span style={{ fontSize: 22, letterSpacing: 1 }}>5G</span>
      <svg width="34" height="20" viewBox="0 0 34 20" fill="none">
        <rect
          x="1"
          y="4"
          width="26"
          height="13"
          rx="3.5"
          stroke={T.onInk}
          strokeWidth="2"
        />
        <rect x="4" y="7" width="18" height="7" rx="1.5" fill={T.onInk} />
        <rect
          x="29"
          y="8"
          width="3"
          height="6"
          rx="1.5"
          fill={T.onInk}
          opacity="0.5"
        />
      </svg>
    </span>
  </div>
);

/** 合规横幅（锁定红，四套主题同值） */
export const ComplianceStrip: React.FC<{ fontSize?: number }> = ({
  fontSize = 23,
}) => (
  <div
    style={{
      margin: "18px 36px 0",
      padding: "14px 20px",
      border: `1px solid ${T.complyBorder}`,
      borderRadius: 12,
      background: T.complyBg,
      color: T.complyStrong,
      fontSize,
      lineHeight: 1.5,
      letterSpacing: "0.02em",
    }}
  >
    <b>「记账局叙」</b>记分工具，不涉及金钱，禁止用于赌博行为
  </div>
);

/** 手机壳：状态栏 + 圆角设备边框 */
export const PhoneShell: React.FC<{
  width?: number;
  delay?: number;
  children: ReactNode;
}> = ({ width = 860, delay = 0, children }) => {
  const p = useEnter(delay, { stiffness: 90 });
  return (
    <div
      style={{
        width,
        borderRadius: 54,
        border: `2px solid ${T.onInkA28}`,
        background: T.ink900,
        boxShadow: `0 48px 90px rgba(0,0,0,0.6), 0 0 140px rgba(201,162,75,0.12)`,
        overflow: "hidden",
        opacity: p,
        scale: `${0.92 + 0.08 * p}`,
        translate: `0px ${(1 - p) * 70}px`,
      }}
    >
      <StatusBar />
      {children}
    </div>
  );
};

/** 计数数字（Oswald tabular） */
export const Count: React.FC<{
  from: number;
  to: number;
  start: number;
  end: number;
  style?: CSSProperties;
}> = ({ from, to, start, end, style }) => {
  const frame = useCurrentFrame();
  const p = interpolate(frame, [start, end], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.out(Easing.cubic),
  });
  const v = Math.round(from + (to - from) * p);
  return (
    <span
      style={{
        fontFamily: NUM,
        fontVariantNumeric: "tabular-nums",
        ...style,
      }}
    >
      {v}
    </span>
  );
};

/** 分数色字（+ 赢色 / − 输色） */
export const Score: React.FC<{
  value: number;
  signed?: boolean;
  style?: CSSProperties;
}> = ({ value, signed = true, style }) => (
  <span
    style={{
      fontFamily: NUM,
      fontVariantNumeric: "tabular-nums",
      color: value > 0 ? T.win : value < 0 ? T.lose : T.dim,
      ...style,
    }}
  >
    {signed && value > 0 ? "+" : ""}
    {value}
  </span>
);
