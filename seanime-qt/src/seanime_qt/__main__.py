"""Application entry point: boots QML and exposes the AppController as ``app``."""

from __future__ import annotations

import gc
import os
import sys
from pathlib import Path

# Quiet Chromium's own native logging (WebGPU/STUN/etc. noise from the AniList
# login page). Must be set before QtWebEngine initializes. Overridable via env.
os.environ.setdefault("QTWEBENGINE_CHROMIUM_FLAGS", "--log-level=3")

from PySide6.QtCore import QCoreApplication, Qt
from PySide6.QtGui import QGuiApplication
from PySide6.QtQml import QQmlApplicationEngine
from PySide6.QtWebEngineQuick import QtWebEngineQuick

from .app_controller import AppController


def _find_qml_dir() -> Path:
    """Locate the qml/ directory whether run from source or installed."""
    here = Path(__file__).resolve()
    candidates = [
        here.parent / "qml",            # installed: seanime_qt/qml
        here.parents[2] / "qml",        # source: seanime-qt/qml
    ]
    for candidate in candidates:
        if (candidate / "Main.qml").exists():
            return candidate
    raise FileNotFoundError(
        "Could not locate Main.qml. Looked in: "
        + ", ".join(str(c) for c in candidates)
    )


def _agent_enabled() -> bool:
    return os.environ.get("SEANIME_QT_AGENT", "").strip().lower() not in ("", "0", "false")


def _start_control_server(
    engine: QQmlApplicationEngine, controller: AppController, app: QGuiApplication
) -> None:
    """Bring up the opt-in agent control server (see seanime_qt.agent)."""
    from .agent import DEFAULT_AGENT_PORT
    from .agent.control_server import ControlServer

    window = engine.rootObjects()[0]
    port = int(os.environ.get("SEANIME_QT_AGENT_PORT", str(DEFAULT_AGENT_PORT)))
    # Parented to the app so it lives for the whole process and is cleaned up on exit.
    server = ControlServer(engine, controller, window, port, parent=app)
    server.start()


def main() -> int:
    # When driven by the agent harness, install log capture before anything else so
    # early QML warnings and import-time errors land in the queryable ring buffer.
    if _agent_enabled():
        from .agent import log_capture

        log_capture.install()

    # QtWebEngine (used by the AniList login page) requires a shared GL context
    # and one-time initialization, both before the application is created.
    QCoreApplication.setAttribute(Qt.ApplicationAttribute.AA_ShareOpenGLContexts)
    QtWebEngineQuick.initialize()

    app = QGuiApplication(sys.argv)
    app.setApplicationName("Seanime-Qt")
    app.setOrganizationName("Seanime")

    controller = AppController()

    engine = QQmlApplicationEngine()
    engine.rootContext().setContextProperty("app", controller)

    qml_dir = _find_qml_dir()
    engine.load(str(qml_dir / "Main.qml"))

    if not engine.rootObjects():
        print("Failed to load QML.", file=sys.stderr)
        return 1

    if _agent_enabled():
        _start_control_server(engine, controller, app)

    try:
        exit_code = app.exec()
    except KeyboardInterrupt:
        exit_code = 0

    # Destroy the QML engine (and every `app`-bound binding) while the controller
    # is still alive, so shutdown doesn't dereference a torn-down context object
    # and spew "Cannot read property ... of null" errors. The event loop has
    # already exited, so deleteLater() would never fire — drop the only Python
    # reference and force a synchronous collection instead, which tears down the
    # engine's QML tree immediately, before `controller` goes away.
    del engine
    gc.collect()
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
