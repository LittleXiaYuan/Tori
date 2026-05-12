from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_speech_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_speech_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SpeechTest(unittest.TestCase):
    def test_speech_namespace_delegates_tts_voices_upload_and_stream_url(self) -> None:
        with patch.object(yunque, "_api_call_bytes", return_value=b"audio") as raw:
            self.assertEqual(yunque.speech.tts("你好", voice="v1", format="wav", emotion="happy"), b"audio")
        raw.assert_called_once_with("POST", "/v1/speech/tts", {"text": "你好", "voice": "v1", "format": "wav", "emotion": "happy"})

        with patch.object(yunque, "_api_call", return_value={"voices": [{"id": "v1"}], "providers": []}) as api_call:
            self.assertEqual(yunque.speech.voices()["voices"][0]["id"], "v1")
        api_call.assert_called_once_with("GET", "/v1/speech/voices")

        with patch.object(yunque, "_api_call_multipart", return_value={"filename": "note.txt", "size": 4}) as upload:
            self.assertEqual(yunque.speech.upload(b"demo", "note.txt", "text/plain")["filename"], "note.txt")
        upload.assert_called_once_with("POST", "/v1/upload", "file", "note.txt", b"demo", content_type="text/plain")

        old_base = yunque._API_BASE
        try:
            yunque._API_BASE = "http://localhost:9090"
            self.assertEqual(yunque.speech.stt_stream_url(language="zh", detect_emotion=True), "ws://localhost:9090/v1/speech/stt/stream?language=zh&detect_emotion=true")
        finally:
            yunque._API_BASE = old_base

    def test_speech_stt_posts_binary_audio(self) -> None:
        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *args):
                return False

            def read(self):
                return b'{"text":"hello","emotion":{"label":"calm"}}'

        with patch.object(yunque.urllib.request, "urlopen", return_value=FakeResponse()) as urlopen:
            result = yunque.speech.stt(b"\x01\x02", language="en", detect_emotion=True)
        self.assertEqual(result["text"], "hello")
        request = urlopen.call_args.args[0]
        self.assertIn("/v1/speech/stt?language=en&detect_emotion=true", request.full_url)
        self.assertEqual(request.data, b"\x01\x02")

    def test_agent_kit_exposes_speech(self) -> None:
        self.assertIs(yunque.create_agent_kit().speech, yunque.speech)


if __name__ == "__main__":
    unittest.main()
