import QtQuick
import QtQuick.Controls

// A themed Button: brand-consistent surface, hover/press/focus states, and a
// pointing-hand cursor — a drop-in replacement for Button (inherits its full
// API, including `checkable`/`checked`, used by the genre picker).
//
// Styling is applied per-control (custom background/contentItem) so the app
// keeps the lightweight Basic Qt Quick Controls style rather than pulling in a
// whole theme engine.
Button {
    id: control

    horizontalPadding: Theme.controlPadding
    verticalPadding: 0
    font.pixelSize: Theme.controlFont

    // Subtle tactile press.
    scale: down ? 0.98 : 1.0
    Behavior on scale { NumberAnimation { duration: Theme.durFast } }

    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }

    contentItem: Text {
        text: control.text
        font: control.font
        color: !control.enabled ? Theme.textMuted
             : control.checked  ? Theme.accentText
             : Theme.text
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
        Behavior on color { ColorAnimation { duration: Theme.durFast } }
    }

    background: Rectangle {
        implicitWidth: 64
        implicitHeight: Theme.controlHeight
        radius: Theme.controlRadius
        color: !control.enabled ? Theme.surface
             : control.checked  ? (control.down ? Theme.accentHover : Theme.accent)
             : control.down      ? Theme.surface
             : control.hovered   ? Qt.lighter(Theme.elevated, 1.3)
             : Theme.elevated
        border.width: control.activeFocus ? 2 : 1
        border.color: control.activeFocus ? Theme.accent
                    : control.checked      ? Theme.accentHover
                    : Theme.border
        Behavior on color { ColorAnimation { duration: Theme.durFast } }
        Behavior on border.color { ColorAnimation { duration: Theme.durFast } }
    }
}
