#!/usr/bin/env python3
"""Run the unmodified official worker serially; drain on TERM without killing login."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import threading
import time

STATE = Path(os.environ.get("REAUTH_STATE_FILE", "/tmp/reauth-supervisor.json"))
STOP = threading.Event()

def write_state(phase):
    temporary = STATE.with_suffix(".tmp")
    temporary.write_text(json.dumps({"pid": os.getpid(), "phase": phase, "updated": time.time()}))
    temporary.replace(STATE)

def health():
    try:
        state = json.loads(STATE.read_text())
        os.kill(int(state["pid"]), 0)
        return 0 if state["phase"] in {"running", "idle"} else 1
    except (OSError, ValueError, KeyError):
        return 1

def main(command=None):
    if sys.argv[1:] == ["--health"]:
        return health()
    signal.signal(signal.SIGTERM, lambda *_: STOP.set())
    signal.signal(signal.SIGINT, lambda *_: STOP.set())
    command = command or [sys.executable, "/app/openai_oauth_reauth_worker.py", "--once"]
    interval = max(1.0, float(os.environ.get("OPENAI_REAUTH_POLL_SECONDS", "5")))
    try:
        while not STOP.is_set():
            write_state("running")
            # A separate process group prevents a terminal/container signal from
            # reaching an in-flight protocol child. Only the supervisor drains.
            child = subprocess.Popen(command, start_new_session=True)
            while child.poll() is None:
                if STOP.is_set():
                    write_state("draining")
                time.sleep(0.2)
            if child.returncode != 0:
                write_state("failed")
                return child.returncode
            write_state("idle")
            STOP.wait(interval)
        return 0
    finally:
        STATE.unlink(missing_ok=True)

if __name__ == "__main__":
    raise SystemExit(main())
