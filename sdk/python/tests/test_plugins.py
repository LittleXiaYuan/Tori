import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class PluginsSDKTests(unittest.TestCase):
    def test_management_lifecycle_file_ui_and_reload_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.plugins.list()
            yunque.plugins.toggle("demo", True)
            yunque.plugins.create("demo", description="Demo", language="python", template="blank", system_prompt="You are Demo", skills=[{"name": "run"}])
            yunque.plugins.delete("demo")
            yunque.plugins.files("demo")
            yunque.plugins.save_file("demo", "handler.py", "print('ok')", plugin="demo")
            yunque.plugins.ui()
            yunque.plugin_ui.ui()
            yunque.plugins.reload()
            yunque.plugins.open_folder("demo")

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/plugins"))
        self.assertEqual(calls[1], ("POST", "/v1/plugins/toggle", {"name": "demo", "enabled": True}))
        self.assertEqual(calls[2], ("POST", "/v1/plugins/create", {"name": "demo", "description": "Demo", "language": "python", "template": "blank", "system_prompt": "You are Demo", "skills": [{"name": "run"}]}))
        self.assertEqual(calls[3], ("DELETE", "/v1/plugins/delete?name=demo"))
        self.assertEqual(calls[4], ("GET", "/v1/plugins/files?name=demo"))
        self.assertEqual(calls[5], ("PUT", "/v1/plugins/files?name=demo", {"file": "handler.py", "content": "print('ok')", "plugin": "demo"}))
        self.assertEqual(calls[6], ("GET", "/v1/plugins/ui"))
        self.assertEqual(calls[7], ("GET", "/v1/plugins/ui"))
        self.assertEqual(calls[8], ("POST", "/v1/plugins/reload"))
        self.assertEqual(calls[9], ("GET", "/v1/plugins/open-folder?name=demo"))

    def test_agent_kit_exposes_plugins_namespace(self):
        self.assertIs(yunque.create_agent_kit().plugins, yunque.plugins)
        self.assertIs(yunque.create_agent_kit().plugin_ui, yunque.plugin_ui)


if __name__ == "__main__":
    unittest.main()
