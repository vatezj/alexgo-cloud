import { fade } from "@remotion/transitions/fade";
import { linearTiming, TransitionSeries } from "@remotion/transitions";
import "./fonts";
import { EndScene } from "./scenes/EndScene";
import { HomeScene } from "./scenes/HomeScene";
import { PainPoint } from "./scenes/PainPoint";
import { RoomScene } from "./scenes/RoomScene";
import { SettleScene } from "./scenes/SettleScene";

/**
 * 记账局叙 · 小红书版推广视频（竖屏 9:16 / 30s）
 *
 * 合规差异（相对微信版 ScorecardPromo）：
 * - 落版：无小程序码、无「微信搜一搜」→ 合规声明卡 + 站内收藏 CTA
 * - 痛点问句「谁欠谁多少」→「谁输谁赢」（去金钱联想）
 * - 首页底注、落版底注改为记分工具口径
 * - 旁白 x1–x5：无站外引流词，末句带「只记分数，不涉及金钱」
 *
 * 结构与微信版一致：120+150+150+270+258 − 4×12 = 900 帧 = 30s @30fps
 */
const FADE = 12;

export const XhsVideo: React.FC = () => {
  return (
    <TransitionSeries>
      <TransitionSeries.Sequence name="01-痛点" durationInFrames={120}>
        <PainPoint
          question="谁输谁赢？"
          vo={{ src: "voice/x1.wav", start: 0, dur: 120 }}
        />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="02-首页" durationInFrames={150}>
        <HomeScene
          footer="免费记分工具 · 只记分数不涉及金钱"
          vo={{ src: "voice/x2.wav", start: 8, dur: 110 }}
        />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="03-记分" durationInFrames={150}>
        <RoomScene vo={{ src: "voice/x3.wav", start: 12, dur: 137, rate: 1.05 }} />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="04-结算" durationInFrames={270}>
        <SettleScene vo={{ src: "voice/x4.wav", start: 20, dur: 202 }} />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="05-落版" durationInFrames={258}>
        <EndScene
          mode="xhs"
          vo={{ src: "voice/x5.wav", start: 115, dur: 142, rate: 1.05 }}
        />
      </TransitionSeries.Sequence>
    </TransitionSeries>
  );
};
