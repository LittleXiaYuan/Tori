#!/usr/bin/env python3
"""Hello World plugin handler for Yunque Agent.

Receives arguments via PLUGIN_ARGS environment variable (JSON).
Prints the result to stdout.
"""
import json
import os

args = json.loads(os.environ.get("PLUGIN_ARGS", "{}"))
name = args.get("name", "World")
style = args.get("style", "casual")

greetings = {
    "formal": f"Good day, {name}. It is a pleasure to assist you.",
    "casual": f"Hey {name}! What's up? 👋",
    "funny": f"Well well well, if it isn't {name}! The legend themselves! 🎉",
}

print(greetings.get(style, greetings["casual"]))
