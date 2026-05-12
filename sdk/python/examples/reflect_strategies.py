"""Print reflection strategy hints through the lightweight Python SDK."""

import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

import yunque  # noqa: E402


def main() -> None:
    stats = yunque.reflect.stats()
    strategies = yunque.reflect.strategies(limit=5)
    print(json.dumps({
        "total_experiences": stats.get("total", 0),
        "recent_7d": stats.get("recent_7d", 0),
        "strategies": strategies,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
