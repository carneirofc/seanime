"""MCP server — exposes the Qt app's control surface to Claude Code.

Runs as its own process (launched over stdio by Claude Code), and does two jobs:

1. **Owns the app lifecycle.** ``app_start`` / ``app_stop`` / ``app_restart``
   spawn and kill the Qt process with ``SEANIME_QT_AGENT=1`` so changes to the
   QML/Python can be picked up with a single restart.
2. **Forwards commands.** Every other tool opens a short-lived TCP connection to
   the in-app control server (see :mod:`seanime_qt.agent.control_server`), sends
   one JSON line, and returns the reply.

Being a dev-only harness in a separate process, it uses the Python standard
library freely.
"""

from __future__ import annotations

import atexit
import base64
import json
import os
import shlex
import socket
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Optional

from mcp.server.fastmcp import FastMCP, Image

from . import DEFAULT_AGENT_PORT

mcp = FastMCP("seanime-qt")

# The running app subprocess, if we started it.
_proc: Optional[subprocess.Popen] = None


# ---- configuration ------------------------------------------------------


def _port() -> int:
    return int(os.environ.get("SEANIME_QT_AGENT_PORT", str(DEFAULT_AGENT_PORT)))


def _project_root() -> Path:
    # .../seanime-qt/src/seanime_qt/agent/mcp_server.py -> seanime-qt
    return Path(__file__).resolve().parents[3]


def _launch_cmd() -> list[str]:
    override = os.environ.get("SEANIME_QT_CMD")
    if override:
        return shlex.split(override)
    # Reuse the exact interpreter running the MCP server (same venv → same PySide6).
    return [sys.executable, "-m", "seanime_qt"]


# ---- socket transport ---------------------------------------------------


def _read_line(sock: socket.socket, timeout: float) -> bytes:
    sock.settimeout(timeout)
    chunks: list[bytes] = []
    while True:
        chunk = sock.recv(65536)
        if not chunk:
            break
        chunks.append(chunk)
        if b"\n" in chunk:
            break
    return b"".join(chunks).split(b"\n", 1)[0]


def _request(cmd: str, timeout: float = 15.0, **args: Any) -> Any:
    payload = json.dumps({"cmd": cmd, **args}).encode("utf-8") + b"\n"
    try:
        with socket.create_connection(("127.0.0.1", _port()), timeout=timeout) as sock:
            sock.sendall(payload)
            line = _read_line(sock, timeout)
    except (ConnectionRefusedError, OSError) as exc:
        raise RuntimeError(
            f"Could not reach the app control server on 127.0.0.1:{_port()} "
            f"({exc}). Is the app running? Call app_start first."
        ) from exc
    if not line:
        raise RuntimeError("empty reply from control server")
    reply = json.loads(line.decode("utf-8"))
    if not reply.get("ok"):
        raise RuntimeError(reply.get("error", "unknown error"))
    return reply.get("result")


# ---- lifecycle ----------------------------------------------------------


def _is_running() -> bool:
    return _proc is not None and _proc.poll() is None


def _probe_port() -> str:
    """Classify the control port: 'ours', 'foreign', or 'free'."""
    try:
        health = _request("health", timeout=1.5)
    except Exception as exc:  # noqa: BLE001 - classify any failure
        # Connection refused → free; anything else answered but wasn't our JSON.
        return "free" if "Is the app running" in str(exc) else "foreign"
    return "ours" if isinstance(health, dict) and health.get("app") == "Seanime-Qt" else "foreign"


def _wait_healthy(timeout: float = 25.0) -> dict:
    deadline = time.time() + timeout
    last_error: Optional[str] = None
    while time.time() < deadline:
        # If we launched it and it already died, stop waiting and surface why.
        if _proc is not None and _proc.poll() is not None:
            raise RuntimeError(
                f"app process exited early with code {_proc.returncode}. "
                "Check get_logs / the terminal for a traceback."
            )
        try:
            return _request("health", timeout=2.0)
        except RuntimeError as exc:
            last_error = str(exc)
            time.sleep(0.4)
    raise RuntimeError(f"app did not become healthy within {timeout:.0f}s ({last_error})")


@mcp.tool()
def app_start() -> dict:
    """Launch the Seanime-Qt app with the agent control server enabled.

    No-op if it is already running. Returns the app's health once it is
    reachable. Inherits the current environment, so any AniList/backend env vars
    set for the MCP server are passed through to the app.
    """
    global _proc
    if _is_running():
        return {"status": "already-running", "health": _request("health")}
    # The port might already be serving: our own app started manually, or an
    # unrelated process squatting on it. Distinguish the two up front.
    probe = _probe_port()
    if probe == "ours":
        return {"status": "already-serving", "health": _request("health")}
    if probe == "foreign":
        raise RuntimeError(
            f"127.0.0.1:{_port()} is occupied by another process that is not "
            "Seanime-Qt. Set SEANIME_QT_AGENT_PORT to a free port and retry."
        )
    env = dict(os.environ)
    env["SEANIME_QT_AGENT"] = "1"
    env["SEANIME_QT_AGENT_PORT"] = str(_port())
    _proc = subprocess.Popen(  # noqa: S603 - launching our own app by design
        _launch_cmd(),
        cwd=str(_project_root()),
        env=env,
    )
    health = _wait_healthy()
    return {"status": "started", "pid": _proc.pid, "health": health}


@mcp.tool()
def app_stop() -> dict:
    """Terminate the app process if the harness started it."""
    global _proc
    if not _is_running():
        _proc = None
        return {"status": "not-running"}
    assert _proc is not None
    _proc.terminate()
    try:
        _proc.wait(timeout=8.0)
    except subprocess.TimeoutExpired:
        _proc.kill()
        _proc.wait(timeout=5.0)
    code = _proc.returncode
    _proc = None
    return {"status": "stopped", "returncode": code}


@mcp.tool()
def app_restart() -> dict:
    """Stop (if running) and start the app again — use to pick up code changes."""
    app_stop()
    return app_start()


# ---- introspection & control -------------------------------------------


@mcp.tool()
def health() -> dict:
    """Return app/Qt versions, the Seanime connection status, and login state."""
    return _request("health")


@mcp.tool()
def dump_tree(object_name: Optional[str] = None, filter: Optional[str] = None, max_depth: int = 40) -> dict:
    """Dump the visual item tree as JSON.

    Each node has objectName, class, window-mapped geometry, visibility, and any
    ``text``. ``object_name`` roots the dump at a named item; ``filter`` keeps
    only nodes whose objectName/text/class contains the substring (plus their
    ancestors) — the fast way to locate something without pixels.
    """
    args: dict[str, Any] = {"maxDepth": max_depth}
    if object_name:
        args["objectName"] = object_name
    if filter:
        args["filter"] = filter
    return _request("tree", **args)


@mcp.tool()
def screenshot(object_name: Optional[str] = None) -> Image:
    """Capture the window (or a named item) as a PNG image."""
    args = {"objectName": object_name} if object_name else {}
    result = _request("screenshot", **args)
    return Image(data=base64.b64decode(result["base64"]), format="png")


@mcp.tool()
def click(object_name: str, button: str = "left") -> dict:
    """Click the centre of a named item (button: left/right/middle)."""
    return _request("click", objectName=object_name, button=button)


@mcp.tool()
def type_text(
    text: str, object_name: Optional[str] = None, clear: bool = False, enter: bool = False
) -> dict:
    """Type text, optionally focusing a named field first, clearing it, and
    pressing Return afterwards (``enter`` — triggers ``onAccepted`` handlers)."""
    args: dict[str, Any] = {"text": text, "clear": clear, "enter": enter}
    if object_name:
        args["objectName"] = object_name
    return _request("type", **args)


@mcp.tool()
def press_key(key: str, modifiers: Optional[list] = None) -> dict:
    """Send a named key to the focused item — the way to test keyboard behaviour.

    Keys: return/enter/tab/backtab/space/escape, left/right/up/down, home/end,
    backspace/delete. Optional ``modifiers``: shift/ctrl/alt/meta. ``tab`` (and
    ``shift``+``backtab``) move focus; the reply includes the new focused item.
    """
    args: dict[str, Any] = {"key": key}
    if modifiers:
        args["modifiers"] = modifiers
    return _request("key", **args)


@mcp.tool()
def active_focus() -> dict:
    """Return the objectName/class of the item that currently holds keyboard focus."""
    return _request("focus")


@mcp.tool()
def accessible(object_name: str) -> dict:
    """Report a named item's accessibility interface (role/name/description) as an
    assistive tool would see it — use to check an element is announced correctly."""
    return _request("accessible", objectName=object_name)


@mcp.tool()
def invoke(object: str, method: str, args: Optional[list] = None) -> dict:
    """Call a slot/method on a named object — the deterministic way to drive state.

    ``object`` is "app" (the AppController) or an item objectName. Examples:
    ``invoke("app", "openAnime", [21])``, ``invoke("app", "searchAnilist", ["naruto"])``,
    ``invoke("app", "loadDiscover")``, ``invoke("app", "refresh")``.
    """
    return _request("invoke", object=object, method=method, args=args or [])


@mcp.tool()
def get_property(object: str, property: str) -> dict:
    """Read a QML property from a named object (or "app")."""
    return _request("getprop", object=object, property=property)


@mcp.tool()
def set_property(object: str, property: str, value: Any) -> dict:
    """Write a QML property on a named object (or "app")."""
    return _request("setprop", object=object, property=property, value=value)


@mcp.tool()
def get_logs(since: int = 0, level: Optional[str] = None) -> dict:
    """Return captured log events (Qt/QML warnings, Python logs, exceptions).

    Pass the ``cursor`` from a previous call as ``since`` to get only new events.
    ``level`` filters to that severity and above (debug/info/warning/error/fatal).
    """
    args: dict[str, Any] = {"since": since}
    if level:
        args["level"] = level
    return _request("logs", **args)


# ---- entry point --------------------------------------------------------


@atexit.register
def _cleanup() -> None:
    # Never leave an orphaned app process behind when the MCP server exits.
    if _is_running():
        assert _proc is not None
        _proc.terminate()
        try:
            _proc.wait(timeout=5.0)
        except subprocess.TimeoutExpired:
            _proc.kill()


def main() -> None:
    mcp.run()


if __name__ == "__main__":
    main()
