#!/usr/bin/env python3
"""记账局叙推广视频 · 旁白生成（MiMo speech-synthesis v2.5）

- 模型：mimo-v2.5-tts（预置音色）
- 目标文本放 role=assistant，导演模式风格指令放 role=user
- API Key：读 ~/.claude/settings.json 的 env.ANTHROPIC_AUTH_TOKEN（系统 Key）
- 用法：python3 gen_tts.py [s1 s2 ...]   不带参数=全部（含 s1_suda 对比样音）
"""
import base64
import json
import pathlib
import sys
import urllib.request

API_URL = "https://api.xiaomimimo.com/v1/chat/completions"
MODEL = "mimo-v2.5-tts"
VOICE = "冰糖"  # 明亮女声（默认）
OUT_DIR = pathlib.Path(__file__).resolve().parent.parent / "public" / "voice"

# 导演模式：角色 / 场景 / 指导 三维度（方案 B · 短句口号体）
STYLE = (
    "【角色】广告片王牌旁白，明亮自信的年轻女声，声音有弹性、有号召力。\n"
    "【场景】为「记账局叙」打牌记账小程序录制30秒朋友圈信息流广告口播，"
    "文案是短句口号体，节奏感是命脉。\n"
    "【指导】语速偏快但咬字极清；每个句号都利落收住、留出节奏空拍；"
    "破折号处语气舒展打开；「一键结清」「一眼看清」干脆有爆发力；"
    "结尾「微信搜一搜」像朋友轻快地怂恿你来一局。"
    "整体：利落 → 自信 → 爆发 → 轻松号召。"
)

# 五幕旁白（对应 PainPoint / Home / Room / Settle / End）· 微信版
LINES = {
    "s1": "散场了。账呢？谁欠谁多少，别猜了。",
    "s2": "开一局。记分，交给记账局叙。",
    "s3": "记一笔。全桌同步，笔笔清楚。",
    "s4": "一键结清，谁给谁多少，一眼看清——剩下的，只有散场的轻松。",
    "s5": "微信搜一搜，记账局叙。散场不散账。",
}

# 小红书版：记分口径、无站外引流词、末句带合规声明
XHS_LINES = {
    "x1": "散场了。谁输谁赢，回头谁还记得？",
    "x2": "开一局。记分，交给记账局叙。",
    "x3": "记一笔。全桌同步，笔笔清楚。",
    "x4": "一键结清，谁记给谁多少，一眼看清——剩下的，只有散场的轻松。",
    "x5": "记账局叙，散场不散账——只记分数，不涉及金钱。",
}

# 对比样音：同文案换茉莉（备选女声）
AUDITIONS = {"s1_moli": (LINES["s1"], "茉莉")}


def load_key() -> str:
    settings = json.loads(
        (pathlib.Path.home() / ".claude" / "settings.json").read_text()
    )
    key = settings.get("env", {}).get("ANTHROPIC_AUTH_TOKEN")
    if not key:
        sys.exit("未找到 ANTHROPIC_AUTH_TOKEN")
    return key


def synth(key: str, text: str, voice: str) -> bytes:
    body = json.dumps(
        {
            "model": MODEL,
            "messages": [
                {"role": "user", "content": STYLE},
                {"role": "assistant", "content": text},
            ],
            "audio": {"format": "wav", "voice": voice},
        }
    ).encode()
    req = urllib.request.Request(
        API_URL,
        data=body,
        headers={
            "Authorization": f"Bearer {key}",
            "Content-Type": "application/json",
        },
    )
    with urllib.request.urlopen(req, timeout=180) as resp:
        data = json.loads(resp.read())
    try:
        b64 = data["choices"][0]["message"]["audio"]["data"]
    except (KeyError, IndexError):
        sys.exit(f"响应异常: {json.dumps(data, ensure_ascii=False)[:500]}")
    return base64.b64decode(b64)


def main() -> None:
    key = load_key()
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    jobs = {}  # name -> (text, voice)
    if len(sys.argv) > 1:
        for k in sys.argv[1:]:
            if k in AUDITIONS:
                jobs[k] = AUDITIONS[k]
            elif k in LINES:
                jobs[k] = (LINES[k], VOICE)
            elif k in XHS_LINES:
                jobs[k] = (XHS_LINES[k], VOICE)
            else:
                sys.exit(f"未知行: {k}")
    else:
        jobs = {k: (v, VOICE) for k, v in LINES.items()}
        jobs.update({k: (v, VOICE) for k, v in XHS_LINES.items()})
        jobs.update(AUDITIONS)
    for name, (text, voice) in jobs.items():
        print(f"→ {name} [{voice}]: {text}")
        wav = synth(key, text, voice)
        out = OUT_DIR / f"{name}.wav"
        out.write_bytes(wav)
        print(f"  已保存 {out} ({len(wav)} bytes)")
    print("完成")


if __name__ == "__main__":
    main()
