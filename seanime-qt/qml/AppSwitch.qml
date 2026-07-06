import QtQuick
import QtQuick.Controls

// A themed Switch: indigo accent track when on, sliding handle, hand cursor.
// Drop-in for Switch.
Switch {
    id: control
    spacing: 8

    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }

    indicator: Rectangle {
        implicitWidth: 40
        implicitHeight: 22
        x: control.leftPadding
        y: parent.height / 2 - height / 2
        radius: height / 2
        color: !control.enabled ? Theme.surface
             : control.checked   ? Theme.accent
             : Theme.elevated
        border.width: 1
        border.color: control.checked ? Theme.accentHover : Theme.border
        Behavior on color { ColorAnimation { duration: Theme.durFast } }

        Rectangle {
            x: control.checked ? parent.width - width - 2 : 2
            y: 2
            width: 18
            height: 18
            radius: 9
            color: control.enabled ? Theme.textStrong : Theme.textMuted
            Behavior on x { NumberAnimation { duration: Theme.durFast; easing.type: Theme.easeStandard } }
        }
    }

    contentItem: Text {
        text: control.text
        font: control.font
        color: control.enabled ? Theme.text : Theme.textMuted
        verticalAlignment: Text.AlignVCenter
        leftPadding: control.indicator.width + control.spacing
    }
}
