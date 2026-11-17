import { fade } from "@remotion/transitions/fade";
import { linearTiming, TransitionSeries } from "@remotion/transitions";
import "./fonts";
import { EndScene } from "./scenes/EndScene";
import { HomeScene } from "./scenes/HomeScene";
import { PainPoint } from "./scenes/PainPoint";
import { RoomScene } from "./scenes/RoomScene";
import { SettleScene } from "./scenes/SettleScene";

/**
 * 记账局叙 · 推广视频（竖屏 9:16 / 30s）
 *
 * 1 痛点 120 → 2 首页 150 → 3 记分 150 → 4 结算高潮 270 → 5 落版 258
 * 4 段 fade 各 12 帧：120+150+150+270+258 − 48 = 900 帧 = 30s @30fps
 */
const FADE = 12;

export const PromoVideo: React.FC = () => {
  return (
    <TransitionSeries>
      <TransitionSeries.Sequence name="01-痛点" durationInFrames={120}>
        <PainPoint />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="02-首页" durationInFrames={150}>
        <HomeScene />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="03-记分" durationInFrames={150}>
        <RoomScene />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="04-结算" durationInFrames={270}>
        <SettleScene />
      </TransitionSeries.Sequence>
      <TransitionSeries.Transition
        presentation={fade()}
        timing={linearTiming({ durationInFrames: FADE })}
      />
      <TransitionSeries.Sequence name="05-落版" durationInFrames={258}>
        <EndScene />
      </TransitionSeries.Sequence>
    </TransitionSeries>
  );
};
