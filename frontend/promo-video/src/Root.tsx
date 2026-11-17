import { Composition, Folder, Still } from "remotion";
import {
  Card01Cover,
  Card02Pain,
  Card03Steps,
  Card04Room,
  Card05Settle,
  Card06Stats,
  Card07Compliance,
  Card08End,
} from "./cards/cards";
import { PromoVideo } from "./PromoVideo";
import { XhsVideo } from "./XhsVideo";

const CARD_W = 1080;
const CARD_H = 1440;

export const RemotionRoot: React.FC = () => {
  return (
    <>
      {/* 微信版：小程序码 + 搜一搜落版 */}
      <Composition
        id="ScorecardPromo"
        component={PromoVideo}
        durationInFrames={900}
        fps={30}
        width={1080}
        height={1920}
      />
      {/* 小红书版：合规声明 + 收藏 CTA，无站外引流 */}
      <Composition
        id="XiaohongshuPromo"
        component={XhsVideo}
        durationInFrames={900}
        fps={30}
        width={1080}
        height={1920}
      />
      {/* 小红书图文卡片（3:4） */}
      <Folder name="XhsCards">
        <Still id="Card01-Cover" component={Card01Cover} width={CARD_W} height={CARD_H} />
        <Still id="Card02-Pain" component={Card02Pain} width={CARD_W} height={CARD_H} />
        <Still id="Card03-Steps" component={Card03Steps} width={CARD_W} height={CARD_H} />
        <Still id="Card04-Room" component={Card04Room} width={CARD_W} height={CARD_H} />
        <Still id="Card05-Settle" component={Card05Settle} width={CARD_W} height={CARD_H} />
        <Still id="Card06-Stats" component={Card06Stats} width={CARD_W} height={CARD_H} />
        <Still id="Card07-Compliance" component={Card07Compliance} width={CARD_W} height={CARD_H} />
        <Still id="Card08-End" component={Card08End} width={CARD_W} height={CARD_H} />
      </Folder>
    </>
  );
};
