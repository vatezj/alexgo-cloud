import { SERIF, SANS, T } from "../theme";
import {
  Bg,
  Capsule,
  Headline,
  PhoneShell,
  Score,
  useEnter,
  Voice,
  type VoiceCfg,
} from "../ui";

const STATS = [
  { v: "12", label: "本月对局" },
  { v: "1,860", label: "累计净胜", accent: true },
  { v: "58%", label: "胜率" },
];

const RECENT = [
  { date: "10月8日", game: "麻将 · 7avw", score: 180 },
  { date: "10月5日", game: "掼蛋 · 3k2s", score: -60 },
];

/** 第二幕：首页 — 扇形 CTA 开房/扫码进房 */
export const HomeScene: React.FC<{
  /** 底部小字（小红书版改为合规口径） */
  footer?: string;
  vo?: VoiceCfg;
}> = ({ footer = "微信小程序 · 记账局叙", vo }) => {
  const cardP = useEnter(30, { stiffness: 90 });
  const cta1P = useEnter(48, { stiffness: 120, damping: 14 });
  const cta2P = useEnter(60, { stiffness: 120, damping: 14 });
  const row1P = useEnter(74);
  const row2P = useEnter(84);
  const v = vo ?? { src: "voice/s2.wav", start: 8, dur: 106 };

  return (
    <Bg>
      {/* 旁白：开一局 */}
      <Voice cfg={v} />
      <Headline
        lines={[{ text: "开一局" }, { text: "记分交给它", accent: true }]}
        delay={8}
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
        <PhoneShell width={860} delay={18}>
          {/* 自绘 nav */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              padding: "14px 36px 4px",
            }}
          >
            <span
              style={{
                fontFamily: SERIF,
                fontWeight: 700,
                fontSize: 34,
                letterSpacing: "0.18em",
                color: T.brass,
              }}
            >
              记账局叙
            </span>
            <Capsule />
          </div>

          {/* 身份牌 */}
          <div
            style={{
              margin: "18px 36px 0",
              padding: "26px 30px",
              borderRadius: 24,
              background: `linear-gradient(165deg, ${T.tile}, ${T.tileShade})`,
              boxShadow: T.shadowTile,
              opacity: cardP,
              scale: `${0.9 + 0.1 * cardP}`,
              translate: `0px ${(1 - cardP) * -30}px`,
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 22 }}>
              <div
                style={{
                  width: 88,
                  height: 88,
                  borderRadius: "50%",
                  background: T.ink600,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontFamily: SERIF,
                  fontWeight: 700,
                  fontSize: 42,
                  color: T.brass,
                }}
              >
                A
              </div>
              <div>
                <div
                  style={{
                    fontFamily: SERIF,
                    fontWeight: 700,
                    fontSize: 40,
                    color: T.jet,
                    letterSpacing: "0.04em",
                  }}
                >
                  Alex星
                </div>
                <div
                  style={{
                    fontSize: 22,
                    color: "rgba(42,39,34,0.55)",
                    marginTop: 4,
                  }}
                >
                  牌桌常客 · 只记分数
                </div>
              </div>
            </div>
            <div
              style={{
                display: "flex",
                marginTop: 24,
                paddingTop: 20,
                borderTop: "1px solid rgba(42,39,34,0.12)",
              }}
            >
              {STATS.map((s, i) => (
                <div
                  key={s.label}
                  style={{
                    flex: 1,
                    textAlign: "center",
                    borderLeft: i === 0 ? "none" : "1px solid rgba(42,39,34,0.12)",
                  }}
                >
                  <div
                    style={{
                      fontFamily: '"Oswald", sans-serif',
                      fontSize: 52,
                      fontWeight: 600,
                      lineHeight: 1.1,
                      color: s.accent ? "#B32D18" : T.jet,
                      fontVariantNumeric: "tabular-nums",
                    }}
                  >
                    {s.v}
                  </div>
                  <div
                    style={{
                      fontSize: 20,
                      color: "rgba(42,39,34,0.55)",
                      marginTop: 4,
                    }}
                  >
                    {s.label}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* 扇形 CTA */}
          <div
            style={{
              margin: "30px 36px 0",
              position: "relative",
              height: 300,
            }}
          >
            <button
              style={{
                position: "absolute",
                top: 0,
                left: "3%",
                width: "94%",
                padding: "30px 0",
                borderRadius: 20,
                border: "none",
                background: `linear-gradient(180deg, ${T.brassHi}, ${T.brass})`,
                boxShadow: `0 6px 0 ${T.brassLo}, 0 18px 30px rgba(0,0,0,0.4)`,
                fontFamily: SERIF,
                fontWeight: 700,
                fontSize: 42,
                letterSpacing: "0.2em",
                color: T.jet,
                rotate: "-1.6deg",
                opacity: cta1P,
                scale: `${0.85 + 0.15 * cta1P}`,
                translate: `0px ${(1 - cta1P) * 40}px`,
              }}
            >
              我要开房
            </button>
            <button
              style={{
                position: "absolute",
                top: 140,
                left: "5%",
                width: "90%",
                padding: "28px 0",
                borderRadius: 20,
                border: `2px solid ${T.tileEmboss}`,
                background: T.onInkA04,
                fontFamily: SERIF,
                fontWeight: 700,
                fontSize: 40,
                letterSpacing: "0.2em",
                color: T.onInk,
                rotate: "1.4deg",
                opacity: cta2P,
                scale: `${0.85 + 0.15 * cta2P}`,
                translate: `0px ${(1 - cta2P) * 40}px`,
              }}
            >
              扫码进房
            </button>
          </div>

          {/* 最近战绩 */}
          <div style={{ margin: "26px 36px 40px" }}>
            <div
              style={{
                fontFamily: SERIF,
                fontSize: 26,
                fontWeight: 500,
                letterSpacing: "0.2em",
                color: T.brass,
                marginBottom: 14,
              }}
            >
              最近战绩
            </div>
            {RECENT.map((r, i) => {
              const p = i === 0 ? row1P : row2P;
              return (
                <div
                  key={r.date}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    padding: "22px 24px",
                    marginBottom: 14,
                    borderRadius: 16,
                    background: T.tile,
                    boxShadow: T.shadowTile,
                    opacity: p,
                    translate: `0px ${(1 - p) * 30}px`,
                  }}
                >
                  <div>
                    <div
                      style={{
                        fontSize: 27,
                        fontWeight: 500,
                        color: T.jet,
                      }}
                    >
                      {r.date} · {r.game}
                    </div>
                    <div
                      style={{
                        fontSize: 20,
                        color: "rgba(42,39,34,0.5)",
                        marginTop: 4,
                      }}
                    >
                      房间流水可回看
                    </div>
                  </div>
                  <Score value={r.score} style={{ fontSize: 44, fontWeight: 600 }} />
                </div>
              );
            })}
          </div>
        </PhoneShell>
      </div>
      {/* 底部小字合规 */}
      <div
        style={{
          position: "absolute",
          bottom: 90,
          left: 0,
          right: 0,
          textAlign: "center",
          fontFamily: SANS,
          fontSize: 26,
          letterSpacing: "0.1em",
          color: T.dim,
        }}
      >
        {footer}
      </div>
    </Bg>
  );
};
