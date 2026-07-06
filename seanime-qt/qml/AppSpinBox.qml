import QtQuick
import QtQuick.Controls

// A themed SpinBox: dark inset field, brand focus ring, and +/- steppers that
// highlight when pressed. Drop-in for SpinBox (respects `editable`).
SpinBox {
    id: control

    implicitHeight: Theme.controlHeight
    font.pixelSize: Theme.controlFont

    contentItem: TextInput {
        text: control.displayText
        font: control.font
        color: Theme.text
        selectionColor: Theme.accent
        selectedTextColor: Theme.accentText
        horizontalAlignment: Qt.AlignHCenter
        verticalAlignment: Qt.AlignVCenter
        readOnly: !control.editable
        validator: control.validator
        inputMethodHints: Qt.ImhFormattedNumbersOnly
    }

    background: Rectangle {
        implicitWidth: 120
        implicitHeight: Theme.controlHeight
        radius: Theme.controlRadius
        color: control.enabled ? Theme.inset : Theme.surface
        border.width: control.activeFocus ? 2 : 1
        border.color: control.activeFocus ? Theme.accent : Theme.border
        Behavior on border.color { ColorAnimation { duration: Theme.durFast } }
    }

    up.indicator: Rectangle {
        x: control.mirrored ? 0 : control.width - width
        height: control.height
        implicitWidth: Theme.controlHeight
        radius: Theme.controlRadius
        color: control.up.pressed ? Theme.surfaceHover : "transparent"
        Behavior on color { ColorAnimation { duration: Theme.durFast } }
        Text {
            text: "+"
            color: control.enabled ? Theme.text : Theme.textMuted
            anchors.centerIn: parent
            font.pixelSize: Theme.fontLg
        }
    }

    down.indicator: Rectangle {
        x: control.mirrored ? control.width - width : 0
        height: control.height
        implicitWidth: Theme.controlHeight
        radius: Theme.controlRadius
        color: control.down.pressed ? Theme.surfaceHover : "transparent"
        Behavior on color { ColorAnimation { duration: Theme.durFast } }
        Text {
            text: "−"
            color: control.enabled ? Theme.text : Theme.textMuted
            anchors.centerIn: parent
            font.pixelSize: Theme.fontLg
        }
    }
}
