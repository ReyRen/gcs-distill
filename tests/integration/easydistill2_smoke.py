"""Run real upstream CLI jobs against an isolated deterministic API fixture.

Run inside the built image with --network none. This checks the runtime contract,
not model quality, and never sends data to a third-party model service.
"""
import copy
import json
import subprocess
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import yaml


class Fixture(BaseHTTPRequestHandler):
    counter = 0

    def log_message(self, *args):
        pass

    def send_json(self, value):
        body = json.dumps(value).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.headers.get("Authorization") != "Bearer fixture":
            self.send_response(401)
            self.end_headers()
            return
        self.send_json({"object": "list", "data": [{"id": "fixture", "object": "model"}]})

    def do_POST(self):
        if self.headers.get("Authorization") != "Bearer fixture":
            self.send_response(401)
            self.end_headers()
            return
        payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        prompt = "\n".join(str(x.get("content", "")) for x in payload.get("messages", []))
        if "<score>" in prompt or "</score>" in prompt:
            score = 1 if "Output ONLY '1'" in prompt else (2 if "BAD_CANDIDATE" in prompt else 8)
            content = f"<score>{score}</score>"
        else:
            Fixture.counter += 1
            marker = "BAD_CANDIDATE" if Fixture.counter % 2 == 0 else "GOOD_CANDIDATE"
            content = f"{marker}: Two plus two equals four. First add two and two, then verify the sum is four."
        self.send_json({"id": "fixture-completion", "object": "chat.completion", "created": 1, "model": "fixture",
                        "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}],
                        "usage": {"prompt_tokens": 12, "completion_tokens": 20, "total_tokens": 32}})


def main():
    server = ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    backend = {"type": "openai", "api_key": "fixture", "model_id": "fixture", "base_url": f"http://127.0.0.1:{server.server_port}/v1"}
    results = []
    with tempfile.TemporaryDirectory(prefix="gcs-distill2-smoke-") as temp:
        root = Path(temp)
        seed = root / "seed.jsonl"
        seed.write_text('\n'.join(json.dumps({"instruction": f"Explain the sum of two and two, example {i}", "problem": "What is 2+2?"}) for i in range(2)), encoding="utf-8")
        cases = [
            ("instruct_distill", "basic/instruct_distill_pai_token.yaml"),
            ("cot_distill", "basic/cot_distill_pai_token.yaml"),
            ("advanced_instruct_distill", "pipeline/advanced_instruct_distill_pai_token.yaml"),
            ("dpo_data_build", "preference/dpo_instruct_pai_token.yaml"),
        ]
        for job, recipe in cases:
            cfg = yaml.safe_load((Path("/opt/easydistill/configs") / recipe).read_text())
            cfg["job_type"] = job
            cfg["backend"] = copy.deepcopy(backend)
            cfg["dataset"] = {"input_path": str(seed), "output_path": str(root / f"{job}.jsonl"), "min_length": 1}
            if job == "advanced_instruct_distill":
                cfg["pipeline"] = [s for s in cfg["pipeline"] if s["stage"] in {"generate", "instruct_eval", "quality_filter", "build_sft"}]
                for s in cfg["pipeline"]:
                    if s["stage"] == "quality_filter":
                        s["config"] = {"min_scores": {"helpfulness": 6}, "require_all_metrics": True, "keep_top_ratio": 1.0}
            for i, stage in enumerate(cfg.get("pipeline", [])):
                stage["output_path"] = str(root / f"{job}_{i}.jsonl")
            path = root / f"{job}.json"
            path.write_text(json.dumps(cfg), encoding="utf-8")
            proc = subprocess.run(["easydistill", "--config", str(path)], capture_output=True, text=True, timeout=90)
            if proc.returncode:
                raise RuntimeError(f"{job}: {proc.stdout}\n{proc.stderr}")
            rows = [json.loads(x) for x in Path(cfg["dataset"]["output_path"]).read_text().splitlines() if x.strip()]
            assert rows, f"{job} produced no records"
            if job == "dpo_data_build":
                assert all(r.get("chosen") and r.get("rejected") for r in rows)
            else:
                assert all(r.get("messages") for r in rows)
            results.append({"job_type": job, "records": len(rows), "status": "passed"})
    server.shutdown()
    print(json.dumps({"result": "EASYDISTILL2_IMAGE_SMOKE_OK", "tests": results}, ensure_ascii=False))


if __name__ == "__main__":
    if len(sys.argv)>1 and sys.argv[1] == "serve":
        ThreadingHTTPServer(("0.0.0.0", int(sys.argv[2]) if len(sys.argv)>2 else 18882), Fixture).serve_forever()
    else:
        main()
