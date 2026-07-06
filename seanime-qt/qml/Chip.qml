import QtQuick
import QtQuick.Controls

// A small rounded pill used for metadata (score, format, genres…). Set
// `interactive` to make it a button — it then hovers and emits `clicked`.
Rectangle {
    id: chip
    property string text: ""
    property color textColor: Theme.textDim
    property color fillColor: Theme.elevated
    property bool interactive: false
    signal clicked()

    implicitWidth: label.implicitWidth + 16
    implicitHeight: 22
    radius: Theme.radiusPill
    color: interactive && hover.hovered ? Theme.accentFill : fillColor
    Behavior on color { ColorAnimation { duration: Theme.durFast } }

    Label {
        id: label
        anchors.centerIn: parent
        text: chip.text
        color: chip.interactive && hover.hovered ? Theme.accentSoft : chip.textColor
        font.pixelSize: Theme.fontXs
    }

    HoverHandler {
        id: hover
        enabled: chip.interactive
        cursorShape: Qt.PointingHandCursor
    }
    TapHandler {
        enabled: chip.interactive
        onTapped: chip.clicked()
    }
}
