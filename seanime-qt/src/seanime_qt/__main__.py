"""Application entry point: boots QML and exposes the AppController as ``app``."""

from __future__ import annotations

import gc
import logging
import os
import sys
from pathlib import Path


def _configure_logging() -> None:
    """Send the app's Python logs to the terminal (stderr).

    Level is controlled by ``SEANIME_QT_LOG_LEVEL`` (default ``INFO``); set it to
    ``DEBUG`` to also see every HTTP request the client makes. This is separate
    from the Qt/QML message capture used by the agent harness.
    """
    level_name = os.environ.get("SEANIME_QT_LOG_LEVEL", "INFO").upper()
    level = getattr(logging, level_name, logging.INFO)
    logging.basicConfig(
        level=level,
        stream=sys.stderr,
        format="%(asctime)s %(levelname)-7s %(name)s: %(message)s",
        datefmt="%H:%M:%S",
    )

# Quiet Chromium's own native logging (WebGPU/STUN/etc. noise from the AniList
# login page). Must be set before QtWebEngine initializes. Overridable via env.
os.environ.setdefault("QTWEBENGINE_CHROMIUM_FLAGS", "--log-level=3")

# Use the Basic Qt Quick Controls style so the app's custom control styling
# (see AppButton/AppToolButton/AppComboBox, which set contentItem/background from
# the Theme tokens) is honoured. The native platform style ignores such
# customization ("current style does not support customization") and draws light
# controls that clash with the dark Theme. Overridable via env. Must be set
# before the QML engine loads any Controls.
os.environ.setdefault("QT_QUICK_CONTROLS_STYLE", "Basic")

from PySide6.QtCore import QCoreApplication, Qt
from PySide6.QtGui import QGuiApplication
from PySide6.QtQml import QQmlApplicationEngine, qmlRegisterType
from PySide6.QtWebEngineQuick import QtWebEngineQuick

from .adult_filter import AdultFilterProxy
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
    # Terminal logging first, so extension/HTTP diagnostics are visible from the
    # very start of the run.
    _configure_logging()
    logging.getLogger("seanime_qt").info("Seanime-Qt starting")

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

    # Let QML instantiate its own adult/safe split proxies (used by MediaCarousel).
    qmlRegisterType(AdultFilterProxy, "Seanime", 1, 0, "AdultFilterProxy")

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
