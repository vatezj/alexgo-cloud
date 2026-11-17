import { loadFont as loadNotoSerifSC } from "@remotion/google-fonts/NotoSerifSC";
import { loadFont as loadNotoSansSC } from "@remotion/google-fonts/NotoSansSC";
import { loadFont as loadOswald } from "@remotion/google-fonts/Oswald";

loadNotoSerifSC("normal", {
  weights: ["500", "700"],
  subsets: ["chinese-simplified", "latin"],
  ignoreTooManyRequestsWarning: true,
});

loadNotoSansSC("normal", {
  weights: ["400", "500"],
  subsets: ["chinese-simplified", "latin"],
  ignoreTooManyRequestsWarning: true,
});

loadOswald("normal", {
  weights: ["500", "600"],
  subsets: ["latin"],
});
