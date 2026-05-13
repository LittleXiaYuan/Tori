import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class PluginsSDKTests(unittest.TestCase):
    def test_management_lifecycle_file_ui_and_reload_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.plugins.list()
            yunque.plugins.toggle("demo", True)
            yunque.plugin_toggle.toggle("demo", True)
            yunque.plugins.create("demo", description="Demo", language="python", template="blank", system_prompt="You are Demo", skills=[{"name": "run"}])
            yunque.plugins.delete("demo")
            yunque.plugins.files("demo")
            yunque.plugin_files.files("demo")
            yunque.plugins.save_file("demo", "handler.py", "print('ok')", plugin="demo")
            yunque.plugin_files.save_file("demo", "handler.py", "print('ok')", plugin="demo")
            yunque.plugins.ui()
            yunque.plugin_ui.ui()
            yunque.plugins.reload()
            yunque.plugin_reload.reload()
            yunque.plugins.open_folder("demo")
            yunque.plugin_folder.open_folder("demo")
            yunque.plugin_search.search("agent", 2)
            yunque.plugin_send.send("webhook", "ops", "hello")
            yunque.plugin_llm.complete([{"role": "user", "content": "hi"}], temperature=0.2, model="demo")
            yunque.plugin_llm.llm("system", "user", model="demo", temperature=0.3)
            yunque.plugin_memory.get("foo")
            yunque.plugin_memory.set("foo", "bar")
            yunque.plugin_memory.delete("foo")
            yunque.plugin_memory.list("f")
            yunque.plugin_memory.search("bar", 3)
            yunque.plugin_knowledge.search("sdk", 2)
            yunque.plugin_knowledge.ingest("content", source="doc", filename="a.md")

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/plugins"))
        self.assertEqual(calls[1], ("POST", "/v1/plugins/toggle", {"name": "demo", "enabled": True}))
        self.assertEqual(calls[2], ("POST", "/v1/plugins/toggle", {"name": "demo", "enabled": True}))
        self.assertEqual(calls[3], ("POST", "/v1/plugins/create", {"name": "demo", "description": "Demo", "language": "python", "template": "blank", "system_prompt": "You are Demo", "skills": [{"name": "run"}]}))
        self.assertEqual(calls[4], ("DELETE", "/v1/plugins/delete?name=demo"))
        self.assertEqual(calls[5], ("GET", "/v1/plugins/files?name=demo"))
        self.assertEqual(calls[6], ("GET", "/v1/plugins/files?name=demo"))
        self.assertEqual(calls[7], ("PUT", "/v1/plugins/files?name=demo", {"file": "handler.py", "content": "print('ok')", "plugin": "demo"}))
        self.assertEqual(calls[8], ("PUT", "/v1/plugins/files?name=demo", {"file": "handler.py", "content": "print('ok')", "plugin": "demo"}))
        self.assertEqual(calls[9], ("GET", "/v1/plugins/ui"))
        self.assertEqual(calls[10], ("GET", "/v1/plugins/ui"))
        self.assertEqual(calls[11], ("POST", "/v1/plugins/reload"))
        self.assertEqual(calls[12], ("POST", "/v1/plugins/reload"))
        self.assertEqual(calls[13], ("GET", "/v1/plugins/open-folder?name=demo"))
        self.assertEqual(calls[14], ("GET", "/v1/plugins/open-folder?name=demo"))
        self.assertEqual(calls[15], ("POST", "/v1/plugin-api/search", {"query": "agent", "limit": 2}))
        self.assertEqual(calls[16], ("POST", "/v1/plugin-api/send", {"channel": "webhook", "target": "ops", "content": "hello", "format": "markdown"}))
        self.assertEqual(calls[17], ("POST", "/v1/plugin-api/llm", {"messages": [{"role": "user", "content": "hi"}], "temperature": 0.2, "model": "demo"}))
        self.assertEqual(calls[18], ("POST", "/v1/plugin-api/llm", {"messages": [{"role": "system", "content": "system"}, {"role": "user", "content": "user"}], "temperature": 0.3, "model": "demo"}))
        self.assertEqual(calls[19], ("POST", "/v1/plugin-api/memory/get", {"key": "foo"}))
        self.assertEqual(calls[20], ("POST", "/v1/plugin-api/memory/set", {"key": "foo", "value": "bar"}))
        self.assertEqual(calls[21], ("POST", "/v1/plugin-api/memory/delete", {"key": "foo"}))
        self.assertEqual(calls[22], ("POST", "/v1/plugin-api/memory/list", {"prefix": "f"}))
        self.assertEqual(calls[23], ("POST", "/v1/plugin-api/memory/search", {"query": "bar", "limit": 3}))
        self.assertEqual(calls[24], ("POST", "/v1/plugin-api/knowledge/search", {"query": "sdk", "limit": 2}))
        self.assertEqual(calls[25], ("POST", "/v1/plugin-api/knowledge/ingest", {"content": "content", "source": "doc", "filename": "a.md"}))

    def test_agent_kit_exposes_plugins_namespace(self):
        self.assertIs(yunque.create_agent_kit().plugins, yunque.plugins)
        self.assertIs(yunque.create_agent_kit().plugin_ui, yunque.plugin_ui)
        self.assertIs(yunque.create_agent_kit().plugin_toggle, yunque.plugin_toggle)
        self.assertIs(yunque.create_agent_kit().plugin_reload, yunque.plugin_reload)
        self.assertIs(yunque.create_agent_kit().plugin_files, yunque.plugin_files)
        self.assertIs(yunque.create_agent_kit().plugin_folder, yunque.plugin_folder)
        self.assertIs(yunque.create_agent_kit().plugin_search, yunque.plugin_search)
        self.assertIs(yunque.create_agent_kit().plugin_send, yunque.plugin_send)
        self.assertIs(yunque.create_agent_kit().plugin_llm, yunque.plugin_llm)
        self.assertIs(yunque.create_agent_kit().plugin_memory, yunque.plugin_memory)
        self.assertIs(yunque.create_agent_kit().plugin_knowledge, yunque.plugin_knowledge)


if __name__ == "__main__":
    unittest.main()
