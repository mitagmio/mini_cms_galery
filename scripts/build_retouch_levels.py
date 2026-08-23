#!/usr/bin/env python3
"""Build two-frame retouch-level WebP loops from foto.zip (original + one variant).

Zip layout: 1.png–4.png are the four retouch stills (light → heavy);
5.png is the shared archival original used as frame A on every card.
"""

from __future__ import annotations

import argparse
import zipfile
from io import BytesIO
from pathlib import Path

import numpy as np
from PIL import Image

CANVAS = (480, 640)  # 3:4 portrait
BG = (246, 241, 241)  # #f6f1f1
HOLD_MS = 900
FADE_MS = 90
FADE_STEPS = 4
QUALITY = 78

# form level -> zip member
LEVEL_STILLS = {
    "1": "1.png",
    "2": "2.png",
    "3": "3.png",
    "4": "4.png",
}
ORIGINAL = "5.png"


def fit_portrait(im: Image.Image, size: tuple[int, int] = CANVAS) -> Image.Image:
    im = im.convert("RGB")
    cw, ch = size
    scale = min(cw / im.width, ch / im.height)
    nw = max(1, int(round(im.width * scale)))
    nh = max(1, int(round(im.height * scale)))
    fitted = im.resize((nw, nh), Image.Resampling.LANCZOS)
    canvas = Image.new("RGB", size, BG)
    canvas.paste(fitted, ((cw - nw) // 2, (ch - nh) // 2))
    return canvas


def blend(a: Image.Image, b: Image.Image, t: float) -> Image.Image:
    aa = np.asarray(a, dtype=np.float32)
    bb = np.asarray(b, dtype=np.float32)
    out = np.clip(aa * (1.0 - t) + bb * t, 0, 255).astype(np.uint8)
    return Image.fromarray(out, "RGB")


def pingpong(orig: Image.Image, variant: Image.Image) -> tuple[list[Image.Image], list[int]]:
    frames: list[Image.Image] = [orig]
    durations: list[int] = [HOLD_MS]
    for i in range(1, FADE_STEPS + 1):
        t = i / (FADE_STEPS + 1)
        frames.append(blend(orig, variant, t))
        durations.append(FADE_MS)
    frames.append(variant)
    durations.append(HOLD_MS)
    for i in range(1, FADE_STEPS + 1):
        t = i / (FADE_STEPS + 1)
        frames.append(blend(variant, orig, t))
        durations.append(FADE_MS)
    return frames, durations


def load_zip(zip_path: Path) -> dict[str, Image.Image]:
    out: dict[str, Image.Image] = {}
    with zipfile.ZipFile(zip_path) as zf:
        names = {Path(n).name: n for n in zf.namelist() if not n.endswith("/")}
        needed = [ORIGINAL, *LEVEL_STILLS.values()]
        missing = [n for n in needed if n not in names]
        if missing:
            raise SystemExit(f"zip missing {missing}; have {sorted(names)}")
        for name in needed:
            raw = zf.read(names[name])
            im = Image.open(BytesIO(raw))
            out[name] = fit_portrait(im)
    return out


def save_webp(path: Path, frames: list[Image.Image], durations: list[int]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    frames[0].save(
        path,
        format="WEBP",
        save_all=True,
        append_images=frames[1:],
        duration=durations,
        loop=0,
        quality=QUALITY,
        method=6,
    )


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--zip", default="/root/sheyanova/front-teamplate/foto.zip")
    ap.add_argument("--out", default="/root/sheyanova/front/assets/theme/rates")
    args = ap.parse_args()
    zip_path = Path(args.zip)
    out_dir = Path(args.out)
    if not zip_path.is_file():
        raise SystemExit(f"zip missing or unreadable: {zip_path}")
    frames = load_zip(zip_path)
    orig = frames[ORIGINAL]
    for level, still in LEVEL_STILLS.items():
        seq, durs = pingpong(orig, frames[still])
        dest = out_dir / f"retouch-level-{level}.webp"
        save_webp(dest, seq, durs)
        print(f"{dest.name}: {dest.stat().st_size} bytes, {len(seq)} frames")


if __name__ == "__main__":
    main()
