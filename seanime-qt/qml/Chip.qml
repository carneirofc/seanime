import QtQuick
import QtQuick.Controls

// A small rounded pill used for metadata (score, format, genres…). Set
// `interactive` to make it a button — it then hovers and emits `clicked`.
Rectangle {
    id: chip
    property string text: ""
    property string icon: ""   // optional leading Tabler icon name (see Icons.qml)
    property color textColor: Theme.textDim
    property color fillColor: Theme.elevated
    property bool interactive: false
    signal clicked()

    implicitWidth: row.implicitWidth + 16
    implicitHeight: 22
    radius: Theme.radiusPill
    color: interactive && hover.hovered ? Theme.accentFill : fillColor
    Behavior on color { ColorAnimation { duration: Theme.durFast } }

    readonly property color _fg: interactive && hover.hovered ? Theme.accentSoft : textColor

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 3
        Icon {
            visible: chip.icon.length > 0
            name: chip.icon
            size: Theme.fontXs
            color: chip._fg
            anchors.verticalCenter: parent.verticalCenter
        }
        Label {
            id: label
            text: chip.text
            color: chip._fg
            font.pixelSize: Theme.fontXs
            anchors.verticalCenter: parent.verticalCenter
        }
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
