"""Print a concise Yunque State Kernel summary as JSON.

Start yunque-agent first so /v1/state is reachable, then run from the repo root:

    python sdk/python/examples/state_snapshot.py
"""

from __future__ import annotations

import json
import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

import yunque  # noqa: E402


def main() -> None:
    snapshot = yunque.state.snapshot()
    summary = {
        "focus": snapshot.get("focus", ""),
        "goal_count": len(snapshot.get("goals") or []),
        "resource_count": len(snapshot.get("resources") or []),
        "recent_actions": [a.get("action", "") for a in snapshot.get("recent_actions") or []],
        "total_skills": (snapshot.get("capabilities") or {}).get("total_skills", 0),
        "unresolved_gaps": (snapshot.get("capabilities") or {}).get("unresolved_gaps", 0),
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
