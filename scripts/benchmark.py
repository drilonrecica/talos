#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Reproducible alpha benchmark harness for Binnacle.

Runs the binnacle binary in deterministic demo mode for a configurable number of
synthetic containers, samples process and application metrics, and emits a JSON
report. Designed for short validation runs locally; the same harness can be used
for longer reference runs on release hardware.
"""

import argparse
import json
import os
import signal
import socket
import statistics
import subprocess
import sys
import tempfile
import threading
import time
import http.cookiejar
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

COOKIE_JAR = http.cookiejar.CookieJar()
URL_OPENER = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(COOKIE_JAR))


def find_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def fetch_json(url: str, timeout: float = 5.0) -> Any:
    with URL_OPENER.open(url, timeout=timeout) as resp:  # noqa: S310
        return json.loads(resp.read().decode("utf-8"))


def post_json(url: str, body: dict[str, Any], timeout: float = 5.0) -> None:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(  # noqa: S310
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with URL_OPENER.open(req, timeout=timeout) as resp:  # noqa: S310
        resp.read()


def authenticate(base_url: str, token: str) -> None:
    post_json(f"{base_url}/api/v1/setup/claim", {"token": token, "username": "benchmark", "password": "benchmark-password-32chars-long"})


def read_proc_stat(pid: int) -> dict[str, int] | None:
    try:
        with open(f"/proc/{pid}/stat", "rb") as f:  # noqa: S108
            parts = f.read().split()
        # fields 14/15 are utime/stime (index 13/14)
        return {
            "utime": int(parts[13]),
            "stime": int(parts[14]),
            "rss_pages": int(parts[23]),
        }
    except Exception:
        return None


def read_self_cpu_times() -> tuple[float, float] | None:
    try:
        with open("/proc/stat", "rb") as f:  # noqa: S108
            for line in f:
                if line.startswith(b"cpu "):
                    parts = line.split()
                    total = sum(int(p) for p in parts[1:])
                    idle = int(parts[4])
                    return float(total), float(idle)
    except Exception:
        return None
    return None


class Sampler:
    def __init__(self, pid: int, base_url: str, interval: float = 1.0) -> None:
        self.pid = pid
        self.base_url = base_url
        self.interval = interval
        self.stopped = threading.Event()
        self.samples: list[dict[str, Any]] = []
        self.thread = threading.Thread(target=self._loop, daemon=True)
        self.clk_tck = os.sysconf(os.sysconf_names.get("SC_CLK_TCK", 2))  # type: ignore[arg-type]

    def start(self) -> None:
        self.thread.start()

    def stop(self) -> None:
        self.stopped.set()
        self.thread.join(timeout=self.interval + 1)

    def _loop(self) -> None:
        prev_cpu_ticks: int | None = None
        prev_at: float | None = None
        while not self.stopped.wait(self.interval):
            at = time.time()
            sample: dict[str, Any] = {"at": at}
            try:
                data = fetch_json(f"{self.base_url}/api/v1/monitor-health", timeout=2.0)
                sample["metrics"] = {m["id"]: m["value"] for m in data.get("metrics", [])}
            except Exception as e:
                sample["metrics_error"] = str(e)

            proc = read_proc_stat(self.pid)
            if proc is not None:
                sample["rss_bytes"] = proc["rss_pages"] * os.sysconf(os.sysconf_names.get("SC_PAGE_SIZE", 30))  # type: ignore[arg-type]
                ticks = proc["utime"] + proc["stime"]
                sample["cpu_time_seconds"] = ticks / self.clk_tck
                if prev_cpu_ticks is not None and prev_at is not None and prev_at < at:
                    delta = ticks - prev_cpu_ticks
                    elapsed = at - prev_at
                    # CPU percent of one core.
                    sample["cpu_percent"] = (delta / self.clk_tck) / elapsed * 100.0
                prev_cpu_ticks = ticks
                prev_at = at
            self.samples.append(sample)


class SSEMeasurer:
    def __init__(self, base_url: str, duration: float = 10.0) -> None:
        self.base_url = base_url
        self.duration = duration
        self.bytes_per_second: float | None = None
        self.thread = threading.Thread(target=self._run, daemon=True)

    def start(self) -> None:
        self.thread.start()

    def join(self) -> None:
        self.thread.join(timeout=self.duration + 5)

    def _run(self) -> None:
        req = urllib.request.Request(  # noqa: S310
            f"{self.base_url}/api/v1/live",
            headers={"Accept": "text/event-stream"},
        )
        start = time.time()
        total = 0
        try:
            with URL_OPENER.open(req, timeout=10) as resp:  # noqa: S310
                while time.time() - start < self.duration:
                    chunk = resp.read(4096)
                    if not chunk:
                        break
                    total += len(chunk)
            elapsed = time.time() - start
            if elapsed > 0:
                self.bytes_per_second = total / elapsed
        except Exception:
            self.bytes_per_second = None


def wait_for_server(base_url: str, timeout: float = 30.0) -> None:
    deadline = time.time() + timeout
    last: Exception | None = None
    while time.time() < deadline:
        try:
            URL_OPENER.open(f"{base_url}/healthz", timeout=2.0)  # noqa: S310
            return
        except Exception as e:
            last = e
            time.sleep(0.2)
    raise RuntimeError(f"server did not become ready: {last}")


def summarize(samples: list[dict[str, Any]]) -> dict[str, Any]:
    def vals(key: str) -> list[float]:
        out: list[float] = []
        for s in samples:
            v = s.get("metrics", {}).get(key)
            if isinstance(v, (int, float)):
                out.append(float(v))
        return out

    def proc(key: str) -> list[float]:
        out: list[float] = []
        for s in samples:
            v = s.get(key)
            if isinstance(v, (int, float)):
                out.append(float(v))
        return out

    def agg(numbers: list[float]) -> dict[str, float | None]:
        if not numbers:
            return {"avg": None, "max": None, "p95": None}
        numbers.sort()
        p95 = numbers[int(len(numbers) * 0.95)] if len(numbers) > 1 else numbers[0]
        return {"avg": statistics.mean(numbers), "max": max(numbers), "p95": p95}

    cpu_run: float | None = None
    if len(samples) >= 2:
        first, last = samples[0].get("cpu_time_seconds"), samples[-1].get("cpu_time_seconds")
        t0, t1 = samples[0].get("at"), samples[-1].get("at")
        if isinstance(first, (int, float)) and isinstance(last, (int, float)) and t1 and t0 and t1 > t0:
            cpu_run = (float(last) - float(first)) / (t1 - t0) * 100.0

    return {
        "process": {
            "cpu_percent": agg(proc("cpu_percent")),
            "cpu_avg_percent_run": cpu_run,
            "rss_bytes": agg(proc("rss_bytes")),
        },
        "write_latency_ms": agg(vals("write_latency")),
        "queue_depth": agg(vals("queue")),
        "dropped_batches": agg(vals("dropped")),
        "go_heap_bytes": agg(vals("heap")),
        "goroutines": agg(vals("goroutines")),
        "database_bytes": agg(vals("database")),
        "wal_bytes": agg(vals("wal")),
        "samples": len(samples),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Binnacle reproducible benchmark harness")
    parser.add_argument("--binary", default="bin/binnacle", help="path to binnacle binary")
    parser.add_argument("--containers", type=int, default=30, help="number of synthetic containers")
    parser.add_argument("--duration", type=int, default=60, help="benchmark duration in seconds")
    parser.add_argument("--warmup", type=int, default=5, help="warmup seconds before sampling")
    parser.add_argument("--sse-duration", type=int, default=10, help="seconds to measure SSE bandwidth")
    parser.add_argument("--output", help="write JSON report to file")
    parser.add_argument("--no-build", action="store_true", help="skip building the binary")
    args = parser.parse_args()

    binary = Path(args.binary).resolve()
    if not args.no_build:
        subprocess.run(["go", "build", "-o", str(binary), "./cmd/binnacle"], check=True)
    if not binary.exists():
        print(f"binary not found: {binary}", file=sys.stderr)
        return 1

    port = find_port()
    base_url = f"http://127.0.0.1:{port}"
    data_dir = Path(tempfile.mkdtemp(prefix="binnacle-benchmark-"))
    env = os.environ.copy()
    env["BINNACLE_LISTEN_ADDRESS"] = f"127.0.0.1:{port}"
    env["BINNACLE_DATA_DIR"] = str(data_dir)
    env["BINNACLE_RUNTIME_DIR"] = str(data_dir / "run")
    env["BINNACLE_DATABASE_PATH"] = str(data_dir / "binnacle.db")
    env["BINNACLE_SETUP_TOKEN"] = "binnacle-benchmark-token-32chars-long"

    cmd = [
        str(binary),
        "--demo",
        "--demo-seed",
        "1",
        "--demo-containers",
        str(args.containers),
    ]

    process = subprocess.Popen(cmd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    try:
        wait_for_server(base_url, timeout=30.0)
        authenticate(base_url, env["BINNACLE_SETUP_TOKEN"])
        if args.warmup:
            time.sleep(args.warmup)

        sampler = Sampler(process.pid, base_url)
        sampler.start()
        sse = SSEMeasurer(base_url, duration=float(args.sse_duration))
        sse.start()

        time.sleep(args.duration)

        sampler.stop()
        sse.join()

        report = {
            "scenario": {
                "containers": args.containers,
                "duration_seconds": args.duration,
                "warmup_seconds": args.warmup,
                "seed": 1,
            },
            "summary": summarize(sampler.samples),
            "sse_bytes_per_second": sse.bytes_per_second,
            "database_size_bytes": None,
            "data_dir": str(data_dir),
        }

        db_path = data_dir / "binnacle.db"
        if db_path.exists():
            report["database_size_bytes"] = db_path.stat().st_size

        if args.output:
            with open(args.output, "w", encoding="utf-8") as f:
                json.dump(report, f, indent=2)
        print(json.dumps(report, indent=2))
        return 0
    finally:
        try:
            process.send_signal(signal.SIGTERM)
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
        # Keep data_dir for inspection unless explicitly requested.


if __name__ == "__main__":
    sys.exit(main())
