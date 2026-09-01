#!/usr/bin/env python3
"""Build two-frame retouch-level WebP loops (original ↔ retouch variant).

Supports foto.zip (1.png–4.png + 5.png original) or beauty_portraits.zip
(01.png–04.png + 05_original.png). Transitions are hard cuts; each state is
shown for HOLD_MS + FADE_MS * FADE_STEPS (same total timing as legacy fades).
"""

from __future__ import annotations

import argparse
import zipfile
from io import BytesIO
from pathlib import Path

from PIL import Image

CANVAS = (480, 640)  # 3:4 portrait
BG = (246, 241, 241)  # #f6f1f1
HOLD_MS = 900
FADE_MS = 90
FADE_STEPS = 4
STATE_MS = HOLD_MS + FADE_MS * FADE_STEPS  # legacy fade window kept as hold
QUALITY = 78

ZIP_LAYOUTS: tuple[dict[str, str], ...] = (
    {
        "original": "05_original.png",
        "1": "01.png",
        "2": "02.png",
        "3": "03.png",
        "4": "04.png",
    },
    {
        "original": "5.png",
        "1": "1.png",
        "2": "2.png",
        "3": "3.png",
        "4": "4.png",
    },
)


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


def pingpong_hard(orig: Image.Image, variant: Image.Image) -> tuple[list[Image.Image], list[int]]:
    return [orig, variant], [STATE_MS, STATE_MS]


def resolve_layout(names: set[str]) -> dict[str, str] | None:
    for layout in ZIP_LAYOUTS:
        if all(name in names for name in layout.values()):
            return layout
    return None


def load_zip(zip_path: Path) -> tuple[Image.Image, dict[str, Image.Image]]:
    with zipfile.ZipFile(zip_path) as zf:
        names = {Path(n).name: n for n in zf.namelist() if not n.endswith("/")}
        layout = resolve_layout(set(names))
        if layout is None:
            raise SystemExit(f"zip layout not recognized; have {sorted(names)}")

        def read(key: str) -> Image.Image:
            raw = zf.read(names[layout[key]])
            return fit_portrait(Image.open(BytesIO(raw)))

        orig = read("original")
        levels = {level: read(level) for level in ("1", "2", "3", "4")}
    return orig, levels


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
    ap.add_argument("--zip", default="/root/sheyanova/beauty_portraits.zip")
    ap.add_argument("--out", default="/root/sheyanova/front/assets/theme/rates")
    args = ap.parse_args()
    zip_path = Path(args.zip)
    out_dir = Path(args.out)
    if not zip_path.is_file():
        raise SystemExit(f"zip missing or unreadable: {zip_path}")
    orig, levels = load_zip(zip_path)
    for level, variant in levels.items():
        seq, durs = pingpong_hard(orig, variant)
        dest = out_dir / f"retouch-level-{level}.webp"
        save_webp(dest, seq, durs)
        print(f"{dest.name}: {dest.stat().st_size} bytes, {len(seq)} frames, {sum(durs)}ms/cycle")


if __name__ == "__main__":
    main()
