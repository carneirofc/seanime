"""Application entry point: boots QML and exposes the AppController as ``app``."""

from __future__ import annotations

import sys
from pathlib import Path

from PySide6.QtGui import QGuiApplication
from PySide6.QtQml import QQmlApplicationEngine

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


def main() -> int:
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

    return app.exec()


if __name__ == "__main__":
    raise SystemExit(main())
