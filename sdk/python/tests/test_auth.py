from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_auth_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_auth_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class AuthTest(unittest.TestCase):
    def test_auth_namespace_delegates_status_login_password_and_token(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/auth/status":
                return {"password_set": True, "authenticated": True}
            if path == "/v1/auth/login":
                return {"token": "jwt-admin", "expires_in": 604800}
            if path == "/v1/auth/set-password":
                return {"status": "ok"}
            return {"token": "jwt-viewer", "type": "Bearer"}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertTrue(yunque.auth.status()["authenticated"])
            self.assertEqual(yunque.auth.login("secret", remember=True)["token"], "jwt-admin")
            self.assertEqual(yunque.auth.set_password("new", current="old")["status"], "ok")
            self.assertEqual(yunque.auth.generate_token(role="viewer")["token"], "jwt-viewer")

        self.assertEqual(calls[0], ("GET", "/v1/auth/status", None))
        self.assertEqual(calls[1], ("POST", "/v1/auth/login", {"password": "secret", "remember": True}))
        self.assertEqual(calls[2], ("POST", "/v1/auth/set-password", {"password": "new", "current": "old"}))
        self.assertEqual(calls[3], ("POST", "/v1/token", {"role": "viewer"}))

    def test_auth_builds_tori_oauth_url_and_agent_kit_exposes_auth(self) -> None:
        self.assertIs(yunque.create_agent_kit().auth, yunque.auth)
        self.assertEqual(yunque.auth.tori_oauth_url(), "http://localhost:9090/v1/auth/oauth/tori")
        self.assertEqual(
            yunque.auth.tori_oauth_url("https://tori.example"),
            "http://localhost:9090/v1/auth/oauth/tori?tori_url=https%3A%2F%2Ftori.example",
        )


if __name__ == "__main__":
    unittest.main()
