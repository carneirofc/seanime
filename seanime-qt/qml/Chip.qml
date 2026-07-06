import QtQuick
import QtQuick.Controls

// A small rounded pill used for metadata (score, format, genres…).
Rectangle {
    id: chip
    property string text: ""
    property color textColor: Theme.textDim
    property color fillColor: Theme.elevated

    implicitWidth: label.implicitWidth + 16
    implicitHeight: 22
    radius: Theme.radiusPill
    color: fillColor

    Label {
        id: label
        anchors.centerIn: parent
        text: chip.text
        color: chip.textColor
        font.pixelSize: Theme.fontXs
    }
}
