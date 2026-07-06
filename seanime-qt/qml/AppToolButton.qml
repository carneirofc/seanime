import QtQuick
import QtQuick.Controls

// A themed ToolButton: flat until hovered, then a subtle rounded surface —
// drop-in for ToolButton with a pointing-hand cursor.
ToolButton {
    id: control

    scale: down ? 0.94 : 1.0
    Behavior on scale { NumberAnimation { duration: Theme.durFast } }

    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }

    contentItem: Text {
        text: control.text
        font: control.font
        color: control.enabled ? Theme.text : Theme.textMuted
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }

    background: Rectangle {
        implicitWidth: 34
        implicitHeight: 34
        radius: Theme.radius
        color: control.down     ? Theme.elevated
             : control.hovered   ? Theme.surfaceHover
             : "transparent"
        border.width: control.activeFocus ? 2 : 0
        border.color: Theme.accent
        Behavior on color { ColorAnimation { duration: Theme.durFast } }
    }
}
