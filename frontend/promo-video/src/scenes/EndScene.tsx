import { Easing, Img, interpolate, staticFile, useCurrentFrame } from "remotion";
import { NUM, SERIF, T } from "../theme";
import { Bg, Count, Headline, useEnter, Voice, type VoiceCfg } from "../ui";

type EndMode = "wechat" | "xhs";

const MONTHS = [
  { m: "7月", v: 320 },
  { m: "8月", v: 180 },
  { m: "9月", v: 520 },
  { m: "10月", v: 420 },
];

const MAX_V = 520;

/** 统计格 */
const StatCard: React.FC<{
  v: string;
  label: string;
  accent?: boolean;
  delay: number;
}> = ({ v, label, accent, delay }) => {
  const p = useEnter(delay);
  return (
    <div
      style={{
        width: 280,
        padding: "34px 0 30px",
        borderRadius: 22,
        background: T.tile,
        boxShadow: T.shadowTile,
        textAlign: "center",
        opacity: p,
        scale: `${0.88 + 0.12 * p}`,
        translate: `0px ${(1 - p) * 40}px`,
      }}
    >
      <div
        style={{
          fontFamily: NUM,
          fontSize: 68,
          fontWeight: 600,
          lineHeight: 1.1,
          color: accent ? T.brassLo : T.jet,
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {v}
      </div>
      <div
        style={{
          fontSize: 24,
          color: "rgba(42,39,34,0.55)",
          marginTop: 8,
          letterSpacing: "0.1em",
        }}
      >
        {label}
      </div>
    </div>
  );
};

/** Phase A：战绩统计 */
const StatsPhase: React.FC = () => {
  const frame = useCurrentFrame();
  const out = interpolate(frame, [108, 132], [1, 0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const bar = (i: number) =>
    interpolate(frame, [46 + i * 10, 76 + i * 10], [0, 1], {
      extrapolateLeft: "clamp",
      extrapolateRight: "clamp",
      easing: Easing.bezier(0.16, 1, 0.3, 1),
    });

  return (
    <div style={{ position: "absolute", inset: 0, opacity: out }}>
      <Headline
        lines={[{ text: "每一局，" }, { text: "都算得清", accent: true }]}
        delay={10}
        size={92}
        top={170}
        exit={100}
      />
      {/* 三格统计 */}
      <div
        style={{
          display: "flex",
          gap: 26,
          justifyContent: "center",
          marginTop: 430,
        }}
      >
        {[
          { v: "32", label: "总局数" },
          { v: "58%", label: "胜率" },
          { v: "+1,240", label: "累计净胜", accent: true },
        ].map((s, i) => (
          <StatCard
            key={s.label}
            v={s.v}
            label={s.label}
            accent={s.accent}
            delay={34 + i * 8}
          />
        ))}
      </div>

      {/* 本月净胜 + 月柱 */}
      <div style={{ margin: "70px 90px 0" }}>
        <div
          style={{
            display: "flex",
            alignItems: "baseline",
            justifyContent: "space-between",
          }}
        >
          <span
            style={{
              fontFamily: SERIF,
              fontSize: 32,
              fontWeight: 700,
              letterSpacing: "0.18em",
              color: T.onInk,
            }}
          >
            2026年10月 · 净胜
          </span>
          <Count
            from={0}
            to={420}
            start={40}
            end={90}
            style={{
              fontSize: 132,
              fontWeight: 600,
              color: T.brass,
              lineHeight: 1,
              textShadow: `0 4px 28px ${T.brassA50}`,
            }}
          />
        </div>
        <div style={{ marginTop: 44 }}>
          {MONTHS.map((mo, i) => (
            <div
              key={mo.m}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 24,
                marginBottom: 26,
              }}
            >
              <span style={{ width: 70, fontSize: 26, color: T.dim }}>
                {mo.m}
              </span>
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
                    width: `${(mo.v / MAX_V) * 100 * bar(i)}%`,
                    height: "100%",
                    borderRadius: 999,
                    background: `linear-gradient(90deg, ${T.brassLo}, ${T.brassHi})`,
                    boxShadow: `0 0 14px ${T.brassA50}`,
                  }}
                />
              </div>
              <ScoreInline value={mo.v} />
            </div>
          ))}
        </div>
        <div
          style={{
            textAlign: "center",
            marginTop: 40,
            fontSize: 25,
            letterSpacing: "0.12em",
            color: T.dim,
          }}
        >
          战绩可回看 · 只记分数，不涉及金钱
        </div>
      </div>
    </div>
  );
};

const ScoreInline: React.FC<{ value: number }> = ({ value }) => (
  <span
    style={{
      width: 110,
      textAlign: "right",
      fontFamily: NUM,
      fontSize: 34,
      fontWeight: 600,
      color: T.win,
      fontVariantNumeric: "tabular-nums",
    }}
  >
    +{value}
  </span>
);

/** Phase B：落版 — wechat=小程序码+搜一搜；xhs=合规声明+收藏 CTA（无站外引流） */
const EndCardPhase: React.FC<{ mode: EndMode }> = ({ mode }) => {
  const frame = useCurrentFrame();
  const inP = interpolate(frame, [112, 140], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.bezier(0.16, 1, 0.3, 1),
  });
  const qrP = useEnter(128, { stiffness: 90 });
  const searchP = useEnter(158, { stiffness: 110 });
  const footP = useEnter(190);

  return (
    <div style={{ position: "absolute", inset: 0, opacity: inP }}>
      {/* 名称 */}
      <div
        style={{
          position: "absolute",
          top: 330,
          left: 0,
          right: 0,
          textAlign: "center",
        }}
      >
        <div
          style={{
            fontFamily: SERIF,
            fontWeight: 700,
            fontSize: 138,
            letterSpacing: "0.16em",
            color: T.brass,
            textShadow: `0 6px 36px ${T.brassA50}`,
          }}
        >
          记账局叙
        </div>
        <div
          style={{
            marginTop: 30,
            fontFamily: SERIF,
            fontWeight: 500,
            fontSize: 50,
            letterSpacing: "0.28em",
            color: T.onInk,
          }}
        >
          散场不散账，一键算清
        </div>
      </div>

      {mode === "wechat" ? (
        <>
          {/* 小程序码 */}
          <div
            style={{
              position: "absolute",
              top: 640,
              left: 0,
              right: 0,
              display: "flex",
              justifyContent: "center",
              opacity: qrP,
              scale: `${0.85 + 0.15 * qrP}`,
              translate: `0px ${(1 - qrP) * 60}px`,
            }}
          >
            <div
              style={{
                padding: 34,
                borderRadius: 30,
                background: `linear-gradient(165deg, ${T.tile}, ${T.tileShade})`,
                boxShadow: `0 24px 60px rgba(0,0,0,0.55), 0 0 0 3px ${T.brass}`,
                transform: "rotate(-1.2deg)",
              }}
            >
              <Img
                src={staticFile("qr-miniapp.jpg")}
                style={{
                  display: "block",
                  width: 400,
                  height: 456,
                  borderRadius: 8,
                }}
              />
              <div
                style={{
                  marginTop: 20,
                  textAlign: "center",
                  fontFamily: SERIF,
                  fontWeight: 700,
                  fontSize: 30,
                  letterSpacing: "0.14em",
                  color: T.jet,
                }}
              >
                扫码直接开一局
              </div>
            </div>
          </div>

          {/* 微信搜索入口 */}
          <div
            style={{
              position: "absolute",
              top: 1360,
              left: 0,
              right: 0,
              display: "flex",
              justifyContent: "center",
              opacity: searchP,
              translate: `0px ${(1 - searchP) * 50}px`,
            }}
          >
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 22,
                width: 860,
                padding: "26px 26px 26px 40px",
                borderRadius: 999,
                background: T.ink700,
                border: `1.5px solid ${T.onInkA28}`,
                boxShadow: "0 18px 40px rgba(0,0,0,0.4)",
                boxSizing: "border-box",
              }}
            >
              <svg width="40" height="40" viewBox="0 0 24 24" fill="none">
                <circle
                  cx="11"
                  cy="11"
                  r="7"
                  stroke={T.dim}
                  strokeWidth="2.4"
                />
                <path
                  d="M16.5 16.5L21 21"
                  stroke={T.dim}
                  strokeWidth="2.4"
                  strokeLinecap="round"
                />
              </svg>
              <span
                style={{
                  flex: 1,
                  fontSize: 34,
                  color: T.onInk,
                  letterSpacing: "0.04em",
                }}
              >
                微信搜一搜：<b style={{ color: T.brass }}>记账局叙</b>
              </span>
              <span
                style={{
                  padding: "16px 44px",
                  borderRadius: 999,
                  background: `linear-gradient(180deg, ${T.brassHi}, ${T.brass})`,
                  boxShadow: `0 4px 0 ${T.brassLo}`,
                  fontFamily: SERIF,
                  fontWeight: 700,
                  fontSize: 32,
                  letterSpacing: "0.1em",
                  color: T.jet,
                }}
              >
                搜索
              </span>
            </div>
          </div>

          {/* 底注 */}
          <div
            style={{
              position: "absolute",
              bottom: 110,
              left: 0,
              right: 0,
              textAlign: "center",
              fontSize: 26,
              letterSpacing: "0.1em",
              color: T.dim,
              opacity: footP,
            }}
          >
            微信小程序 · 免费使用 · 只记分数，不涉及金钱
          </div>
        </>
      ) : (
        <>
          {/* 合规声明卡（小红书版：无二维码、无站外引流） */}
          <div
            style={{
              position: "absolute",
              top: 660,
              left: 0,
              right: 0,
              display: "flex",
              justifyContent: "center",
              opacity: qrP,
              scale: `${0.9 + 0.1 * qrP}`,
              translate: `0px ${(1 - qrP) * 50}px`,
            }}
          >
            <div
              style={{
                width: 860,
                padding: "44px 48px",
                borderRadius: 28,
                background: T.complyBg,
                border: `1.5px solid ${T.complyBorder}`,
                boxSizing: "border-box",
                textAlign: "center",
                boxShadow: "0 24px 60px rgba(0,0,0,0.4)",
              }}
            >
              <div
                style={{
                  fontFamily: SERIF,
                  fontWeight: 700,
                  fontSize: 34,
                  letterSpacing: "0.3em",
                  color: T.complyStrong,
                }}
              >
                合规声明
              </div>
              <div
                style={{
                  margin: "26px 0",
                  padding: "24px 0",
                  borderTop: `1.5px dashed ${T.complyBorder}`,
                  borderBottom: `1.5px dashed ${T.complyBorder}`,
                  fontSize: 31,
                  lineHeight: 1.9,
                  color: T.onInk,
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
                  fontSize: 44,
                  letterSpacing: "0.14em",
                  color: T.brass,
                  textShadow: `0 4px 24px ${T.brassA50}`,
                }}
              >
                只记分数，不涉及金钱
              </div>
            </div>
          </div>

          {/* 站内收藏 CTA（小红书原生互动，无站外引流） */}
          <div
            style={{
              position: "absolute",
              top: 1280,
              left: 0,
              right: 0,
              display: "flex",
              justifyContent: "center",
              opacity: searchP,
              translate: `0px ${(1 - searchP) * 50}px`,
            }}
          >
            <div
              style={{
                padding: "30px 70px",
                borderRadius: 999,
                background: `linear-gradient(180deg, ${T.brassHi}, ${T.brass})`,
                boxShadow: `0 6px 0 ${T.brassLo}, 0 18px 40px rgba(0,0,0,0.45)`,
                fontFamily: SERIF,
                fontWeight: 700,
                fontSize: 40,
                letterSpacing: "0.16em",
                color: T.jet,
              }}
            >
              ♥ 收藏视频 · 开局随手记
            </div>
          </div>

          {/* 底注 */}
          <div
            style={{
              position: "absolute",
              bottom: 110,
              left: 0,
              right: 0,
              textAlign: "center",
              fontSize: 26,
              letterSpacing: "0.1em",
              color: T.dim,
              opacity: footP,
            }}
          >
            记账局叙 · 免费记分工具 · 娱乐使用
          </div>
        </>
      )}
    </div>
  );
};

/** 第五幕：战绩收束 → 落版 */
export const EndScene: React.FC<{ mode?: EndMode; vo?: VoiceCfg }> = ({
  mode = "wechat",
  vo,
}) => {
  const v = vo ?? { src: "voice/s5.wav", start: 115, dur: 120 };
  return (
    <Bg>
      {/* 旁白：随落版出现 */}
      <Voice cfg={v} />
      <StatsPhase />
      <EndCardPhase mode={mode} />
    </Bg>
  );
};
