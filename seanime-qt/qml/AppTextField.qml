import QtQuick
import QtQuick.Controls

// A themed TextField: dark inset field, brand focus ring, and readable
// placeholder/selection colors. Drop-in for TextField.
TextField {
    id: control

    implicitHeight: Theme.controlHeight
    font.pixelSize: Theme.controlFont
    color: Theme.text
    placeholderTextColor: Theme.textMuted
    selectionColor: Theme.accent
    selectedTextColor: Theme.accentText
    leftPadding: Theme.controlPadding
    rightPadding: Theme.controlPadding
    verticalAlignment: TextInput.AlignVCenter

    background: Rectangle {
        implicitWidth: 120
        implicitHeight: Theme.controlHeight
        radius: Theme.controlRadius
        color: control.enabled ? Theme.inset : Theme.surface
        border.width: control.activeFocus ? 2 : 1
        border.color: control.activeFocus ? Theme.accent
                    : control.hovered     ? Theme.borderStrong
                    : Theme.border
        Behavior on border.color { ColorAnimation { duration: Theme.durFast } }
    }
}
