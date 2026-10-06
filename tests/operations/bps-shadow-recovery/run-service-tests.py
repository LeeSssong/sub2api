"""Run selected BPS tests with real production files and their existing test helpers.
Full service package tests remain blocked by unrelated baseline compilation errors.
"""
import json
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
BACKEND = ROOT / "upstream/sub2api/backend"
package = json.loads(subprocess.check_output(
    ["go", "list", "-tags=unit", "-json", "./internal/service"], cwd=BACKEND))
tests = json.loads((HERE / "service-test-files.json").read_text())
files = ["internal/service/" + name for name in package["GoFiles"] + tests]
raise SystemExit(subprocess.run(
    ["go", "test", "-tags=unit", *files, "-run", "ExcelBPS|BPSProbe", "-count=1"],
    cwd=BACKEND).returncode)
