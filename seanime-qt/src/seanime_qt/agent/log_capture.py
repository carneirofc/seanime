"""Log capture — a ring buffer fed by every diagnostic channel in the app.

Installs three sinks so the ``logs`` control command can return a unified,
queryable stream that Claude Code can poll while debugging:

- ``qInstallMessageHandler`` — Qt/QML warnings and ``console.log`` from QML.
- a ``logging`` handler on the root logger — anything the Python code logs.
- ``sys.excepthook`` / ``threading.excepthook`` — uncaught exceptions.

Each captured event gets a monotonically increasing ``seq`` so callers can ask
for "everything since cursor N" without re-reading the whole buffer. Events are
still forwarded to the original sinks (stderr) so normal terminal use is
unaffected.
"""

from __future__ import annotations

import logging
import sys
import threading
import time
import traceback
from collections import deque
from typing import Any, Optional

from PySide6.QtCore import QtMsgType, qInstallMessageHandler

# QtMsgType -> level string.
_QT_LEVEL = {
    QtMsgType.QtDebugMsg: "debug",
    QtMsgType.QtInfoMsg: "info",
    QtMsgType.QtWarningMsg: "warning",
    QtMsgType.QtCriticalMsg: "error",
    QtMsgType.QtFatalMsg: "fatal",
}


class LogBuffer:
    """A bounded, sequence-tagged store of log events."""

    def __init__(self, capacity: int = 2000) -> None:
        self._events: deque[dict[str, Any]] = deque(maxlen=capacity)
        self._seq = 0
        self._lock = threading.Lock()

    def add(self, level: str, source: str, message: str) -> None:
        with self._lock:
            self._seq += 1
            self._events.append(
                {
                    "seq": self._seq,
                    "time": time.time(),
                    "level": level,
                    "source": source,
                    "message": message,
                }
            )

    def since(self, cursor: int = 0, level: Optional[str] = None) -> list[dict[str, Any]]:
        with self._lock:
            events = [e for e in self._events if e["seq"] > cursor]
        if level:
            wanted = _levels_at_least(level)
            events = [e for e in events if e["level"] in wanted]
        return events

    @property
    def cursor(self) -> int:
        with self._lock:
            return self._seq


_ORDER = ["debug", "info", "warning", "error", "fatal"]


def _levels_at_least(level: str) -> set[str]:
    try:
        idx = _ORDER.index(level)
    except ValueError:
        return set(_ORDER)
    return set(_ORDER[idx:])


# Module-level singleton, shared by the control server.
_buffer = LogBuffer()
_installed = False


def buffer() -> LogBuffer:
    return _buffer


class _BufferLoggingHandler(logging.Handler):
    def emit(self, record: logging.LogRecord) -> None:
        try:
            msg = self.format(record)
        except Exception:  # pragma: no cover - formatting must never crash logging
            msg = record.getMessage()
        _buffer.add(record.levelname.lower(), "python", msg)


def install() -> None:
    """Install all sinks. Idempotent; safe to call once at startup."""
    global _installed
    if _installed:
        return
    _installed = True

    # ---- Qt / QML messages ----
    previous_handler = qInstallMessageHandler(None)  # fetch current, then chain

    def _qt_handler(msg_type, context, message):  # noqa: ANN001 - Qt signature
        level = _QT_LEVEL.get(msg_type, "info")
        source = "qml" if context and context.category == "qml" else "qt"
        where = ""
        if context and context.file:
            where = f" ({context.file}:{context.line})"
        _buffer.add(level, source, f"{message}{where}")
        if previous_handler is not None:
            previous_handler(msg_type, context, message)
        else:
            print(f"[{level}] {message}{where}", file=sys.stderr)

    qInstallMessageHandler(_qt_handler)

    # ---- Python logging ----
    handler = _BufferLoggingHandler()
    handler.setFormatter(logging.Formatter("%(name)s: %(message)s"))
    root = logging.getLogger()
    root.addHandler(handler)
    if root.level == logging.NOTSET or root.level > logging.INFO:
        root.setLevel(logging.INFO)

    # ---- uncaught exceptions ----
    prev_excepthook = sys.excepthook

    def _excepthook(exc_type, exc, tb):  # noqa: ANN001 - sys.excepthook signature
        _buffer.add(
            "error",
            "python",
            "Uncaught exception:\n" + "".join(traceback.format_exception(exc_type, exc, tb)),
        )
        prev_excepthook(exc_type, exc, tb)

    sys.excepthook = _excepthook

    if hasattr(threading, "excepthook"):
        prev_thread_hook = threading.excepthook

        def _thread_excepthook(args):  # noqa: ANN001 - threading.excepthook signature
            _buffer.add(
                "error",
                "python",
                "Uncaught thread exception:\n"
                + "".join(
                    traceback.format_exception(
                        args.exc_type, args.exc_value, args.exc_traceback
                    )
                ),
            )
            prev_thread_hook(args)

        threading.excepthook = _thread_excepthook
