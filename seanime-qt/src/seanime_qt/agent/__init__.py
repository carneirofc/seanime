"""Agent harness — an opt-in automation surface for driving the Qt app.

This package is *development-only*. It is never imported unless the app is
launched with ``SEANIME_QT_AGENT=1`` (the app process) or the MCP server is
started explicitly. Because it is a dev tool rather than shipped UI code, it is
free to use the Python standard library (``socket``, ``json``, ``logging``);
the "Qt-only, ports cleanly to C++" rule applies to the application, not to
this harness.

Two pieces:

- :mod:`seanime_qt.agent.control_server` runs *inside* the Qt process (on the
  GUI thread) and exposes a newline-delimited JSON command socket.
- :mod:`seanime_qt.agent.mcp_server` runs as a separate process, owns the app
  subprocess lifecycle, and translates MCP tool calls into socket commands so
  Claude Code can drive the UI.
"""

from __future__ import annotations

DEFAULT_AGENT_PORT = 43299
