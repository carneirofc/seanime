import QtQuick
import QtQuick.Controls

// A themed ComboBox: brand-consistent field, dropdown arrow, and a themed popup
// list with hover/selection highlighting and a fade-in. Drop-in for ComboBox —
// preserves textRole/valueRole behaviour used across the search and editors.
ComboBox {
    id: control

    horizontalPadding: Theme.controlPadding
    verticalPadding: 0
    font.pixelSize: Theme.controlFont

    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }

    // One row in the dropdown.
    delegate: ItemDelegate {
        width: ListView.view ? ListView.view.width : 200
        highlighted: control.highlightedIndex === index
        contentItem: Text {
            text: control.textRole
                  ? (Array.isArray(control.model) ? modelData[control.textRole] : model[control.textRole])
                  : modelData
            color: highlighted ? Theme.accentText : Theme.text
            font: control.font
            elide: Text.ElideRight
            verticalAlignment: Text.AlignVCenter
        }
        background: Rectangle {
            radius: Theme.radiusSm
            color: highlighted ? Theme.accent
                 : hovered     ? Theme.surfaceHover
                 : "transparent"
            Behavior on color { ColorAnimation { duration: Theme.durFast } }
        }
    }

    indicator: Icon {
        x: control.width - width - control.rightPadding
        y: control.topPadding + (control.availableHeight - height) / 2
        name: "chevron-down"
        size: Theme.fontMd
        color: Theme.textMuted
    }

    contentItem: Text {
        leftPadding: 2
        rightPadding: control.indicator.width + 6
        text: control.displayText
        font: control.font
        color: control.enabled ? Theme.text : Theme.textMuted
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }

    background: Rectangle {
        implicitWidth: 120
        implicitHeight: Theme.controlHeight
        radius: Theme.controlRadius
        color: control.down    ? Theme.surface
             : control.hovered ? Qt.lighter(Theme.elevated, 1.2)
             : Theme.elevated
        border.width: (control.activeFocus || control.popup.visible) ? 2 : 1
        border.color: (control.activeFocus || control.popup.visible) ? Theme.accent : Theme.border
        Behavior on color { ColorAnimation { duration: Theme.durFast } }
        Behavior on border.color { ColorAnimation { duration: Theme.durFast } }
    }

    popup: Popup {
        y: control.height + 2
        width: control.width
        implicitHeight: Math.min(contentItem.implicitHeight + 8, 280)
        padding: 4

        enter: Transition { NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durFast } }
        exit: Transition { NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Theme.durFast } }

        contentItem: ListView {
            clip: true
            implicitHeight: contentHeight
            model: control.popup.visible ? control.delegateModel : null
            currentIndex: control.highlightedIndex
            ScrollIndicator.vertical: ScrollIndicator {}
        }

        background: Rectangle {
            color: Theme.surface
            radius: Theme.controlRadius
            border.color: Theme.border
        }
    }
}
