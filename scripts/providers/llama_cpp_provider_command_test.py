import importlib.util
import pathlib
import unittest


module_path = pathlib.Path(__file__).with_name("llama_cpp_provider_command.py")
spec = importlib.util.spec_from_file_location("llama_cpp_provider_command", module_path)
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)


class LlamaCppProviderCommandTest(unittest.TestCase):
    def test_canonical_conversation_preserves_roles_and_latest_user_tail(self):
        payload = adapter.build_chat_payload({
            "protocol": adapter.PROTOCOL,
            "model": "local-model",
            "conversation": [
                {"role": "system", "text": "Stable Genesis prefix"},
                {"role": "user", "text": "Earlier request"},
                {"role": "assistant", "text": "Earlier answer"},
                {"role": "assistant", "tool_calls": [{"tool_call_id": "call_1", "name": "shell_exec", "arguments": {"command": "pwd"}}]},
                {"role": "tool", "tool_call_id": "call_1", "text": "D:/repo"},
                {"role": "user", "text": "Latest user request"},
            ],
            "input_items": [{"kind": "skill_index_context", "text": "must not flatten"}],
            "tool_manifest": [{"name": "shell_exec", "description": "run", "input_schema": {"type": "object"}}],
        })

        self.assertEqual(payload["messages"], [
            {"role": "system", "content": "Stable Genesis prefix"},
            {"role": "user", "content": "Earlier request"},
            {"role": "assistant", "content": "Earlier answer"},
            {"role": "assistant", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "shell_exec", "arguments": "{\"command\":\"pwd\"}"}}]},
            {"role": "tool", "tool_call_id": "call_1", "content": "D:/repo"},
            {"role": "user", "content": "Latest user request"},
        ])
        self.assertEqual(payload["tools"][0]["function"]["name"], "shell_exec")


if __name__ == "__main__":
    unittest.main()
