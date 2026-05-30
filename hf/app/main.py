#!/usr/bin/env python3
"""Launcher for the MatrixCloud Hugging Face Space.

HF runs this as the container entrypoint. It normalises the environment for a
Space (writable data dir, listening port that matches `app_port`) and then
hands off to the real `matrix-runtime` binary via exec, so the Go process
becomes PID 1's child with no extra layer.
"""
import os
import sys

# HF maps external traffic to the container's app_port (7860 by default). Honor
# $PORT if the platform provides one, else fall back to 7860.
port = os.environ.get("PORT") or os.environ.get("MATRIX_RUNTIME_PORT") or "7860"
os.environ["MATRIX_RUNTIME_PORT"] = port

# The Space filesystem is largely read-only; /tmp is writable. Durable data
# (accounts, sessions) should be in Postgres via MATRIXCLOUD_DATABASE_URL.
data_dir = os.environ.setdefault("MATRIX_RUNTIME_DATA_DIR", "/tmp/matrixcloud")
try:
    os.makedirs(data_dir, exist_ok=True)
except OSError as exc:  # pragma: no cover - defensive
    print(f"warning: could not create data dir {data_dir}: {exc}", file=sys.stderr)

mode = os.environ.get("MATRIX_RUNTIME_MODE", "cloud-worker")
print(f"MatrixCloud Space launching: mode={mode} port={port} data_dir={data_dir}", flush=True)

# Replace this process with the runtime binary.
os.execvp("matrix-runtime", ["matrix-runtime", "--mode", mode])
