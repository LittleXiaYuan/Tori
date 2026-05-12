from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_projects_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_projects_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ProjectsNamespaceTest(unittest.TestCase):
    def test_projects_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/projects":
                if method == "GET":
                    return {"projects": [{"id": "p1", "name": "云雀", "repo_path": "C:/repo"}]}
                return {"id": "p1", "name": body["name"], "repo_path": body["repo_path"]}
            if path == "/v1/projects/detail?id=p1":
                return {"id": "p1", "name": "云雀+", "repo_path": "C:/repo", "description": "Agent"}
            if path == "/v1/projects/remove":
                return {"status": "deleted"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.projects.list()["projects"][0]["id"], "p1")
            self.assertEqual(yunque.projects.create("云雀", "C:/repo", default_caps=["read"])["repo_path"], "C:/repo")
            self.assertEqual(yunque.projects.detail("p1")["name"], "云雀+")
            self.assertEqual(yunque.projects.update("p1", {"description": "Agent"})["description"], "Agent")
            self.assertEqual(yunque.projects.remove("p1")["status"], "deleted")

        self.assertEqual(calls[1], ("POST", "/v1/projects", {"name": "云雀", "repo_path": "C:/repo", "default_caps": ["read"]}))
        self.assertEqual(calls[3], ("PUT", "/v1/projects/detail?id=p1", {"description": "Agent"}))
        self.assertIs(yunque.create_agent_kit().projects, yunque.projects)


if __name__ == "__main__":
    unittest.main()
