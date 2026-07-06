import QtQuick
import QtQuick.Controls

// A themed ToolButton: flat until hovered, then a subtle rounded surface —
// drop-in for ToolButton with a pointing-hand cursor.
ToolButton {
    id: control

    // Optional Tabler icon name (see Icons.qml); when set, drawn instead of text.
    property string iconName: ""

    font.pixelSize: Theme.controlFont

    scale: down ? 0.94 : 1.0
    Behavior on scale { NumberAnimation { duration: Theme.durFast } }

    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }

    contentItem: Item {
        implicitWidth: control.iconName.length > 0 ? toolIcon.implicitWidth : toolText.implicitWidth
        implicitHeight: control.iconName.length > 0 ? toolIcon.implicitHeight : toolText.implicitHeight
        Icon {
            id: toolIcon
            visible: control.iconName.length > 0
            anchors.centerIn: parent
            name: control.iconName
            size: Theme.controlFont
            color: control.enabled ? Theme.text : Theme.textMuted
        }
        Text {
            id: toolText
            visible: control.iconName.length === 0
            anchors.centerIn: parent
            text: control.text
            font: control.font
            color: control.enabled ? Theme.text : Theme.textMuted
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
            elide: Text.ElideRight
        }
    }

    background: Rectangle {
        implicitWidth: Theme.controlHeight
        implicitHeight: Theme.controlHeight
        radius: Theme.controlRadius
        color: control.down     ? Theme.elevated
             : control.hovered   ? Theme.surfaceHover
             : "transparent"
        border.width: control.activeFocus ? 2 : 0
        border.color: Theme.accent
        Behavior on color { ColorAnimation { duration: Theme.durFast } }
    }
}
