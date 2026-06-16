"""Shared helpers for the ZK age-verification demos.

This module is the *orchestration* layer. It does NO cryptography itself — it
drives three things and narrates what happens:

  1. the Go ZK prover      (go run zk/prover/main.go ...)   -> makes a proof
  2. the blockchain client (ageverifyd tx / query ...)      -> talks to the node
  3. the CometBFT RPC      (http://localhost:26657)         -> reads chain state

Standard library only — no pip installs needed.
"""

import base64
import json
import subprocess
import sys
import time
import urllib.request

# Fixed infrastructure config (you rarely touch these)
CHAIN_ID = "ageverify"
RPC = "http://localhost:26657"          # CometBFT RPC (read chain state)
NODE = "tcp://localhost:26657"          # same node, for the CLI
KEYRING = ["--keyring-backend", "test"]
WORKDIR = "/workspace"                  # repo root inside the container

# When True, every external call prints the exact command being run and the
# raw response it got back. This is the "prove it's real" switch: flip it on
# for a live demo / Q&A so the audience sees the actual ageverifyd calls and
# the node's real JSON; flip it off for clean slide screenshots.
SHOW_COMMANDS = True

# Pretty printing
_G, _R, _Y, _C, _D, _N = (
    "\033[0;32m", "\033[0;31m", "\033[1;33m", "\033[0;36m", "\033[0;90m", "\033[0m",
)


def step(n, title):
    print(f"\n{_Y}━━ STEP {n}: {title} {_N}")


def ok(msg):
    print(f"{_G}✓  {msg}{_N}")


def fail(msg):
    print(f"{_R}✗  {msg}{_N}")
    sys.exit(1)


def info(key, value):
    print(f"   {_C}{key:<16}{_N} {value}")


def note(msg):
    """Greyed-out explanation of the blockchain concept being shown."""
    print(f"   {_D}{msg}{_N}")


def banner(title):
    width = max(56, len(title) + 2)
    line = "═" * width
    print(f"\n{_C}╔{line}╗{_N}")
    print(f"{_C}║{_N} {title:<{width - 2}} {_C}║{_N}")
    print(f"{_C}╚{line}╝{_N}")


# Verbose-mode plumbing: print the real command + raw response
def _trunc(arg, limit=28):
    """Shorten long base64 blobs (proof/witness) so the command stays readable
    while still showing it is a real argument."""
    s = str(arg)
    if len(s) > limit:
        return f"{s[:18]}…<{len(s)} chars>"
    return s


def _show_cmd(args):
    if SHOW_COMMANDS:
        print(f"   {_D}$ {' '.join(_trunc(a) for a in args)}{_N}")


def _show_resp(obj):
    """Print a raw response (dict or str), dimmed, so the audience sees the
    node's actual JSON rather than just our narration."""
    if not SHOW_COMMANDS:
        return
    text = obj if isinstance(obj, str) else json.dumps(obj, indent=2)
    for line in text.splitlines():
        print(f"   {_D}│ {line}{_N}")


# Subprocess + RPC plumbing
def run(args, check=True):
    """Run a command in the repo root and capture stdout/stderr."""
    _show_cmd(args)
    proc = subprocess.run(
        args, cwd=WORKDIR, capture_output=True, text=True
    )
    if check and proc.returncode != 0:
        fail(f"command failed: {' '.join(args)}\n{proc.stderr.strip()}")
    return proc


def ageverifyd(*args, check=True):
    return run(["ageverifyd", *args], check=check)


def rpc(path, quiet=False):
    """GET a CometBFT RPC endpoint and parse the JSON response.

    `quiet=True` suppresses the printed command — used by polling loops so
    they don't spam the same curl line dozens of times."""
    if not quiet:
        _show_cmd(["curl", "-s", RPC + path])
    with urllib.request.urlopen(RPC + path, timeout=5) as resp:
        return json.loads(resp.read().decode())


def require_chain_live():
    try:
        rpc("/status", quiet=True)
    except Exception:
        fail(f"Chain not reachable at {RPC}. Start it with:\n"
             f"   ageverifyd start --minimum-gas-prices 0stake")


def chain_status():
    return rpc("/status")["result"]


def wait_for_tx(txhash, timeout=25):
    """Poll until a tx is committed in a block. Returns the tx_result dict
    (code, log, gas_used, events) plus the block height, or None on timeout."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            res = rpc(f"/tx?hash=0x{txhash}", quiet=True)
            if "result" in res:
                r = res["result"]
                tx_result = r["tx_result"]
                tx_result["height"] = r.get("height")
                _show_cmd(["curl", "-s", f"{RPC}/tx?hash=0x{txhash}"])
                _show_resp({
                    "height": tx_result.get("height"),
                    "code": tx_result.get("code", 0),
                    "gas_used": tx_result.get("gas_used"),
                    "log": tx_result.get("log", ""),
                })
                return tx_result
        except Exception:
            pass  # not committed yet — keep polling
        time.sleep(1)
    return None


# Domain helpers
def address_of(account):
    return ageverifyd("keys", "show", account, "-a", *KEYRING).stdout.strip()


def generate_proof(year, month, day, min_age=18, current=None):
    """Run the Go prover locally and return its JSON output as a dict.

    `current` may be a (year, month, day) tuple to forge a proof against a
    date other than today. Raises ProofRefused if the prover cannot satisfy
    the circuit (e.g. the person is younger than min_age)."""
    args = ["go", "run", "zk/prover/main.go",
            "--year", str(year), "--month", str(month), "--day", str(day),
            "--min-age", str(min_age)]
    if current is not None:
        cy, cm, cd = current
        args += ["--current-year", str(cy),
                 "--current-month", str(cm),
                 "--current-day", str(cd)]
    proc = run(args, check=False)
    if proc.returncode != 0:
        raise ProofRefused(proc.stderr.strip())
    return json.loads(proc.stdout)


def submit_proof(account, proof):
    """Broadcast a MsgSubmitAgeProof. `proof` is the prover's JSON dict.
    Returns the broadcast result dict (txhash + mempool code)."""
    return _submit(account, proof["proof"], proof["public_witness"], proof["current_date"])


def submit_raw(account, proof_b64, witness_b64, date):
    """Like submit_proof but with explicit fields (for crafted attacks)."""
    return _submit(account, proof_b64, witness_b64, date)


def _submit(account, proof_b64, witness_b64, date):
    proc = ageverifyd(
        "tx", "ageverify", "submit-age-proof", proof_b64, witness_b64, date,
        "--from", account, *KEYRING,
        "--chain-id", CHAIN_ID, "--node", NODE,
        "--broadcast-mode", "sync", "--yes", "--output", "json",
        check=False,
    )
    try:
        result = json.loads(proc.stdout)
    except json.JSONDecodeError:
        fail(f"could not parse broadcast response:\n{proc.stdout}\n{proc.stderr}")
    _show_resp({"txhash": result.get("txhash"),
                "code": result.get("code", 0),
                "raw_log": result.get("raw_log", "")})
    return result


def query_status(address):
    """Query on-chain verification status. Returns {'verified': bool, ...}."""
    proc = ageverifyd(
        "query", "ageverify", "verification-status", address,
        "--node", NODE, "--output", "json", check=False,
    )
    try:
        result = json.loads(proc.stdout)
    except json.JSONDecodeError:
        return {"verified": False, "verified_at": ""}
    _show_resp(result)
    return result


def find_event(tx_result, event_type):
    """Return the attributes of the first event of the given type, decoded."""
    for ev in tx_result.get("events", []) or []:
        if ev.get("type") == event_type:
            out = {}
            for attr in ev.get("attributes", []):
                k = _maybe_b64(attr.get("key", ""))
                v = _maybe_b64(attr.get("value", ""))
                out[k] = v
            return out
    return {}


def _maybe_b64(s):
    """CometBFT versions differ on whether event attrs are base64. Decode if
    it cleanly round-trips, otherwise return the original string."""
    if not isinstance(s, str) or s == "":
        return s
    try:
        decoded = base64.b64decode(s, validate=True)
        text = decoded.decode("utf-8")
        if base64.b64encode(decoded).decode() == s:
            return text
    except Exception:
        pass
    return s


class ProofRefused(Exception):
    """Raised when the prover cannot generate a proof (statement is false)."""
