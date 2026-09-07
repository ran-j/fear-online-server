"""Extract the game's reference-data catalogs from the .Arch01 files into CSVs
that the catalog package embeds.

Each catalog .Arch01 (d3/e3/i3/a3/j3) is a CSV whose every byte except CR/LF is
XOR'd with 0x09.
"""

import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
GAME = os.path.abspath(os.path.join(SCRIPT_DIR, "..", "..", ".."))
FEAR = os.path.join(GAME, "FEAR_Online")
OUT = os.path.join(SCRIPT_DIR, "..", "internal", "catalog", "data")

ARCHIVES = {
    "d3.Arch01": "GameItem.csv",
    "e3.Arch01": "MapInfo.csv",
    "i3.Arch01": "RewardItem.csv",
    "a3.Arch01": "ClassInfo.csv",
    "j3.Arch01": "Recipe.csv",
}


def decode(data):
    return bytes(b if b in (0x0D, 0x0A) else b ^ 0x09 for b in data)


def main():
    os.makedirs(OUT, exist_ok=True)
    for archive, csvname in ARCHIVES.items():
        raw = open(os.path.join(FEAR, archive), "rb").read()
        text = decode(raw).decode("latin-1")
        with open(os.path.join(OUT, csvname), "w", encoding="utf-8", newline="") as handle:
            handle.write(text)
        rows = len([ln for ln in text.splitlines() if ln.strip()]) - 1
        print(f"{archive} -> {csvname} ({rows} rows)")


if __name__ == "__main__":
    main()
