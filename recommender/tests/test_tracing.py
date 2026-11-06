import unittest

from app.chat.tracing import RedactedLangSmithTrace


class RedactedTraceTests(unittest.TestCase):
    def test_trace_sends_only_bounded_metadata_and_hides_inputs_outputs(self):
        class Client:
            call = None

            def create_run(self, **kwargs):
                self.call = kwargs

        trace = RedactedLangSmithTrace.__new__(RedactedLangSmithTrace)
        trace._client = Client()
        trace._project = "test"
        trace.record(route="[REDACTED]", request_chars=90000, product_count=90,
                     warnings=["timeout", "phone-0912345678", "x" * 100], latency_ms=-1)
        call = trace._client.call
        self.assertTrue(call["hide_inputs"])
        self.assertTrue(call["hide_outputs"])
        self.assertEqual(call["inputs"], {"route": "unknown", "request_chars": 1000})
        self.assertEqual(call["outputs"], {"product_count": 10, "warning_codes": ["timeout", "x" * 64]})
        self.assertEqual(call["extra"]["metadata"]["latency_ms"], 0)
        self.assertNotIn("phone-0912345678", repr(call))


if __name__ == "__main__":
    unittest.main()
