import QtQuick
import QtQuick.Controls

// A ToolButton with a pointing-hand cursor on hover. Drop-in for ToolButton.
ToolButton {
    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }
}
