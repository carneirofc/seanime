"""Agent Control Server — an in-process automation surface for the Qt app.

Enabled only when the app is launched with ``SEANIME_QT_AGENT=1``. It listens on
``127.0.0.1:<port>`` (default :data:`~seanime_qt.agent.DEFAULT_AGENT_PORT`,
override with ``SEANIME_QT_AGENT_PORT``) and speaks newline-delimited JSON: one
request object per line, one reply object per line.

    -> {"cmd": "click", "objectName": "connectButton"}\\n
    <- {"ok": true, "result": {...}}\\n

The server is a :class:`QTcpServer` created on the **GUI thread**, so every
command handler runs inside the Qt event loop. That is what makes it safe to
walk the QML object tree, grab the window, and synthesize input events directly
— no cross-thread marshalling required.

Commands
--------
``health``      app/Qt versions and the current connection status.
``tree``        JSON dump of the visual item tree (objectName, class, geometry,
                visibility, text), optionally filtered/depth-limited.
``screenshot``  PNG of the window (or a named item), base64-encoded.
``click``       ``QTest`` mouse click at the centre of a named item.
``type``        focus a named item (optional) and ``QTest`` type text.
``invoke``      call a slot / method on a named object, e.g. ``app.openAnime``.
``getprop`` /
``setprop``     read / write a QML property on a named object.
``logs``        events from the shared log ring buffer since a cursor.
"""

from __future__ import annotations

import base64
import sys
from typing import Any, Callable, Optional

from PySide6.QtCore import (
    QBuffer,
    QByteArray,
    QIODevice,
    QJsonDocument,
    QObject,
    QPoint,
    QPointF,
    Qt,
)
from PySide6.QtNetwork import QHostAddress, QTcpServer, QTcpSocket
from PySide6.QtQuick import QQuickItem, QQuickWindow
from PySide6.QtTest import QTest

from . import log_capture

_MOUSE_BUTTONS = {
    "left": Qt.MouseButton.LeftButton,
    "right": Qt.MouseButton.RightButton,
    "middle": Qt.MouseButton.MiddleButton,
}

# Named keys for the ``key`` command — enough to exercise keyboard navigation
# and activation (Tab order, Enter/Space, arrow keys, Escape).
_KEYS = {
    "return": Qt.Key.Key_Return,
    "enter": Qt.Key.Key_Enter,
    "tab": Qt.Key.Key_Tab,
    "backtab": Qt.Key.Key_Backtab,
    "space": Qt.Key.Key_Space,
    "escape": Qt.Key.Key_Escape,
    "left": Qt.Key.Key_Left,
    "right": Qt.Key.Key_Right,
    "up": Qt.Key.Key_Up,
    "down": Qt.Key.Key_Down,
    "home": Qt.Key.Key_Home,
    "end": Qt.Key.Key_End,
    "backspace": Qt.Key.Key_Backspace,
    "delete": Qt.Key.Key_Delete,
}

_MODIFIERS = {
    "shift": Qt.KeyboardModifier.ShiftModifier,
    "ctrl": Qt.KeyboardModifier.ControlModifier,
    "control": Qt.KeyboardModifier.ControlModifier,
    "alt": Qt.KeyboardModifier.AltModifier,
    "meta": Qt.KeyboardModifier.MetaModifier,
}


class ControlServer(QObject):
    """Listens for JSON commands and drives the running QML UI."""

    def __init__(
        self,
        engine: QObject,
        controller: QObject,
        window: QObject,
        port: int,
        parent: Optional[QObject] = None,
    ) -> None:
        super().__init__(parent)
        self._engine = engine
        self._controller = controller
        self._window = window
        self._port = port
        self._server = QTcpServer(self)
        self._buffers: dict[QTcpSocket, QByteArray] = {}
        self._server.newConnection.connect(self._on_new_connection)

        self._handlers: dict[str, Callable[[dict], Any]] = {
            "health": self._cmd_health,
            "tree": self._cmd_tree,
            "screenshot": self._cmd_screenshot,
            "click": self._cmd_click,
            "type": self._cmd_type,
            "key": self._cmd_key,
            "focus": self._cmd_focus,
            "accessible": self._cmd_accessible,
            "invoke": self._cmd_invoke,
            "getprop": self._cmd_getprop,
            "setprop": self._cmd_setprop,
            "logs": self._cmd_logs,
        }

    # ---- lifecycle -------------------------------------------------------

    def start(self) -> bool:
        if not self._server.listen(QHostAddress(QHostAddress.SpecialAddress.LocalHost), self._port):
            message = (
                f"Control server failed to listen on 127.0.0.1:{self._port}: "
                f"{self._server.errorString()} — the port may be in use. "
                f"Set SEANIME_QT_AGENT_PORT to a free port."
            )
            log_capture.buffer().add("error", "agent", message)
            # The buffer is only reachable *through* this socket, so also shout to
            # stderr where the launcher (and any human) can actually see it.
            print(f"[agent] {message}", file=sys.stderr, flush=True)
            return False
        log_capture.buffer().add(
            "info", "agent", f"Control server listening on 127.0.0.1:{self._port}"
        )
        return True

    # ---- connection handling --------------------------------------------

    def _on_new_connection(self) -> None:
        while self._server.hasPendingConnections():
            socket = self._server.nextPendingConnection()
            self._buffers[socket] = QByteArray()
            socket.readyRead.connect(lambda s=socket: self._on_ready_read(s))
            socket.disconnected.connect(lambda s=socket: self._on_disconnected(s))

    def _on_disconnected(self, socket: QTcpSocket) -> None:
        self._buffers.pop(socket, None)
        socket.deleteLater()

    def _on_ready_read(self, socket: QTcpSocket) -> None:
        buf = self._buffers.get(socket)
        if buf is None:
            return
        buf.append(socket.readAll())
        # Process every complete (newline-terminated) request in the buffer.
        while True:
            idx = buf.indexOf(b"\n")
            if idx < 0:
                break
            line = bytes(buf.left(idx))
            buf.remove(0, idx + 1)
            self._handle_line(socket, line)

    def _handle_line(self, socket: QTcpSocket, line: bytes) -> None:
        try:
            request = _json_loads(line)
            if not isinstance(request, dict):
                raise ValueError("request must be a JSON object")
            cmd = request.get("cmd")
            handler = self._handlers.get(cmd)
            if handler is None:
                raise ValueError(f"unknown command: {cmd!r}")
            result = handler(request)
            reply = {"ok": True, "result": result}
        except Exception as exc:  # noqa: BLE001 - report every failure to the caller
            reply = {"ok": False, "error": f"{type(exc).__name__}: {exc}"}
        socket.write(_json_dumps(reply) + b"\n")
        socket.flush()

    # ---- object resolution ----------------------------------------------

    def _resolve_object(self, name: str) -> QObject:
        """Resolve a named object for invoke/getprop/setprop."""
        if name in ("app", "controller"):
            return self._controller
        if name in ("window", "root"):
            return self._window
        obj = self._window.findChild(QObject, name)
        if obj is not None:
            return obj
        # Fall back to the visual tree for dynamically-created (delegate) items.
        item = _find_item_by_name(self._quick_window().contentItem(), name)
        if item is None:
            raise LookupError(f"no object named {name!r}")
        return item

    def _resolve_item(self, name: str) -> QQuickItem:
        """Resolve a named visual item for click/type/screenshot.

        Walks the live visual tree (``childItems``) rather than ``findChild`` so
        it reliably reaches dynamically-created delegates (Repeater/GridView),
        matching exactly what the ``tree`` command reports.
        """
        item = _find_item_by_name(self._quick_window().contentItem(), name)
        if item is None:
            raise LookupError(f"no visual item named {name!r}")
        return item

    def _quick_window(self) -> QQuickWindow:
        win = self._window
        if isinstance(win, QQuickWindow):
            return win
        # ApplicationWindow exposes its backing QQuickWindow via the "window" property.
        backing = win.property("window")
        if isinstance(backing, QQuickWindow):
            return backing
        raise TypeError("root object is not a QQuickWindow")

    # ---- command handlers -----------------------------------------------

    def _cmd_health(self, _req: dict) -> dict:
        from PySide6 import __version__ as pyside_version
        from PySide6.QtCore import qVersion

        return {
            "app": "Seanime-Qt",
            "pyside": pyside_version,
            "qt": qVersion(),
            "connectionStatus": self._controller.property("connectionStatus"),
            "username": self._controller.property("username"),
            "libraryCount": self._controller.property("libraryCount"),
            "port": self._port,
        }

    def _cmd_tree(self, req: dict) -> dict:
        root_name = req.get("objectName")
        max_depth = int(req.get("maxDepth", 40))
        text_filter = (req.get("filter") or "").lower()

        if root_name:
            root_item = self._resolve_item(root_name)
        else:
            root_item = self._quick_window().contentItem()

        node = _describe_item(root_item, self._quick_window(), max_depth)
        if text_filter:
            node = _prune_tree(node, text_filter)
        return {"tree": node or {}}

    def _cmd_screenshot(self, req: dict) -> dict:
        window = self._quick_window()
        image = window.grabWindow()
        if image.isNull():
            raise RuntimeError("grabWindow() returned a null image")

        object_name = req.get("objectName")
        if object_name:
            item = self._resolve_item(object_name)
            top_left = item.mapToScene(QPointF(0, 0))
            rect = image.rect().intersected(
                _rect_from(
                    int(top_left.x()),
                    int(top_left.y()),
                    int(item.width()),
                    int(item.height()),
                )
            )
            if rect.isEmpty():
                raise RuntimeError(f"item {object_name!r} has no visible area to capture")
            image = image.copy(rect)

        byte_array = QByteArray()
        buffer = QBuffer(byte_array)
        buffer.open(QIODevice.OpenModeFlag.WriteOnly)
        if not image.save(buffer, "PNG"):
            raise RuntimeError("failed to encode screenshot as PNG")
        buffer.close()
        return {
            "format": "png",
            "width": image.width(),
            "height": image.height(),
            "base64": base64.b64encode(bytes(byte_array)).decode("ascii"),
        }

    def _cmd_click(self, req: dict) -> dict:
        item = self._resolve_item(_require(req, "objectName"))
        button = _MOUSE_BUTTONS.get((req.get("button") or "left").lower())
        if button is None:
            raise ValueError(f"unknown button: {req.get('button')!r}")
        centre = item.mapToScene(QPointF(item.width() / 2, item.height() / 2)).toPoint()
        window = self._quick_window()
        QTest.mouseClick(window, button, Qt.KeyboardModifier.NoModifier, centre)
        return {"clicked": req["objectName"], "pos": [centre.x(), centre.y()]}

    def _cmd_type(self, req: dict) -> dict:
        text = _require(req, "text")
        object_name = req.get("objectName")
        window = self._quick_window()
        if object_name:
            item = self._resolve_item(object_name)
            item.forceActiveFocus()
            if req.get("clear"):
                # Replace any existing content deterministically before typing.
                item.setProperty("text", "")
        # keyClicks(str) only accepts a QWidget; for a QQuickWindow we send one
        # key click per character (which routes through the focused QML item).
        for ch in text:
            QTest.keyClick(window, ch)
        if req.get("enter"):
            QTest.keyClick(window, Qt.Key.Key_Return)
        return {"typed": text, "objectName": object_name, "enter": bool(req.get("enter"))}

    def _cmd_key(self, req: dict) -> dict:
        name = _require(req, "key").lower()
        key = _KEYS.get(name)
        if key is None:
            raise ValueError(
                f"unknown key {name!r}; known: {', '.join(sorted(_KEYS))}"
            )
        modifier = Qt.KeyboardModifier.NoModifier
        for mod in req.get("modifiers") or []:
            flag = _MODIFIERS.get(str(mod).lower())
            if flag is None:
                raise ValueError(f"unknown modifier: {mod!r}")
            modifier |= flag
        # Tab focus is delivered to the window; keep it consistent for all keys so
        # the event routes to whatever item currently holds focus.
        window = self._quick_window()
        QTest.keyClick(window, key, modifier)
        return {"key": name, "focus": _active_focus(window)}

    def _cmd_focus(self, _req: dict) -> dict:
        return _active_focus(self._quick_window())

    def _cmd_accessible(self, req: dict) -> dict:
        """Report an item's accessibility interface as an assistive tool sees it."""
        from PySide6.QtGui import QAccessible

        item = self._resolve_item(_require(req, "objectName"))
        iface = QAccessible.queryAccessibleInterface(item)
        if iface is None:
            return {"objectName": req["objectName"], "available": False}
        role = iface.role()
        state = iface.state()
        return {
            "objectName": req["objectName"],
            "available": True,
            "role": role.name if hasattr(role, "name") else int(role),
            "name": iface.text(QAccessible.Text.Name),
            "description": iface.text(QAccessible.Text.Description),
            "focusable": bool(getattr(state, "focusable", False)),
        }

    def _cmd_invoke(self, req: dict) -> dict:
        obj = self._resolve_object(_require(req, "object"))
        method_name = _require(req, "method")
        args = req.get("args") or []
        if not isinstance(args, list):
            raise ValueError("args must be a list")
        method = getattr(obj, method_name, None)
        if method is None or not callable(method):
            raise LookupError(f"{req['object']!r} has no callable {method_name!r}")
        result = method(*args)
        return {"invoked": f"{req['object']}.{method_name}", "result": _jsonable(result)}

    def _cmd_getprop(self, req: dict) -> dict:
        obj = self._resolve_object(_require(req, "object"))
        name = _require(req, "property")
        return {"property": name, "value": _jsonable(obj.property(name))}

    def _cmd_setprop(self, req: dict) -> dict:
        obj = self._resolve_object(_require(req, "object"))
        name = _require(req, "property")
        value = req.get("value")
        if not obj.setProperty(name, value):
            # setProperty returns False for a non-existent (dynamic) property; still
            # report it rather than silently succeeding.
            raise RuntimeError(f"could not set property {name!r} on {req['object']!r}")
        return {"property": name, "value": _jsonable(obj.property(name))}

    def _cmd_logs(self, req: dict) -> dict:
        since = int(req.get("since", 0))
        level = req.get("level")
        events = log_capture.buffer().since(since, level)
        return {"cursor": log_capture.buffer().cursor, "events": events}


# ---- helpers ------------------------------------------------------------


def _active_focus(window: QQuickWindow) -> dict:
    """Describe the item that currently holds keyboard focus."""
    item = window.activeFocusItem()
    if item is None:
        return {"objectName": None, "class": None}
    return {
        "objectName": item.objectName() or None,
        "class": item.metaObject().className(),
    }


def _find_item_by_name(item: QQuickItem, name: str) -> Optional[QQuickItem]:
    """Depth-first search of the visual tree for an item with ``objectName``."""
    if item.objectName() == name:
        return item
    for child in item.childItems():
        if isinstance(child, QQuickItem):
            found = _find_item_by_name(child, name)
            if found is not None:
                return found
    return None


def _describe_item(item: QQuickItem, window: QQuickWindow, depth: int) -> dict:
    top_left = item.mapToScene(QPointF(0, 0))
    node: dict[str, Any] = {
        "objectName": item.objectName(),
        "class": item.metaObject().className(),
        "visible": bool(item.isVisible()),
        "enabled": bool(item.isEnabled()),
        "x": round(top_left.x(), 1),
        "y": round(top_left.y(), 1),
        "width": round(item.width(), 1),
        "height": round(item.height(), 1),
    }
    text = item.property("text")
    if isinstance(text, str) and text:
        node["text"] = text
    if depth > 0:
        children = [
            _describe_item(child, window, depth - 1)
            for child in item.childItems()
            if isinstance(child, QQuickItem)
        ]
        if children:
            node["children"] = children
    return node


def _node_matches(node: dict, needle: str) -> bool:
    return (
        needle in (node.get("objectName") or "").lower()
        or needle in (node.get("text") or "").lower()
        or needle in (node.get("class") or "").lower()
    )


def _prune_tree(node: dict, needle: str) -> Optional[dict]:
    """Keep nodes matching ``needle`` and any ancestor leading to a match."""
    kept_children = [
        pruned
        for child in node.get("children", [])
        if (pruned := _prune_tree(child, needle)) is not None
    ]
    if _node_matches(node, needle) or kept_children:
        result = dict(node)
        if kept_children:
            result["children"] = kept_children
        else:
            result.pop("children", None)
        return result
    return None


def _rect_from(x: int, y: int, w: int, h: int):
    from PySide6.QtCore import QRect

    return QRect(x, y, w, h)


def _require(req: dict, key: str) -> Any:
    if key not in req or req[key] in (None, ""):
        raise ValueError(f"missing required field: {key!r}")
    return req[key]


def _jsonable(value: Any) -> Any:
    """Best-effort conversion of a returned value into something JSON-safe."""
    if value is None or isinstance(value, (bool, int, float, str)):
        return value
    if isinstance(value, (list, tuple)):
        return [_jsonable(v) for v in value]
    if isinstance(value, dict):
        return {str(k): _jsonable(v) for k, v in value.items()}
    return repr(value)


def _json_dumps(obj: Any) -> bytes:
    """Serialise using Qt's JSON so the payload matches the app's own conventions."""
    doc = QJsonDocument.fromVariant(obj)
    return bytes(doc.toJson(QJsonDocument.JsonFormat.Compact))


def _json_loads(data: bytes) -> Any:
    doc = QJsonDocument.fromJson(QByteArray(data))
    if doc.isNull():
        raise ValueError("invalid JSON request")
    # toVariant() yields native Python types (dict/list/str/…), not QJson* wrappers.
    return doc.toVariant()
