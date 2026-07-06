import QtQuick
import QtQuick.Controls

// A small rounded pill used for metadata (score, format, genres…).
Rectangle {
    id: chip
    property string text: ""
    property color textColor: "#c8c8d4"
    property color fillColor: "#22222c"

    implicitWidth: label.implicitWidth + 16
    implicitHeight: 22
    radius: 11
    color: fillColor

    Label {
        id: label
        anchors.centerIn: parent
        text: chip.text
        color: chip.textColor
        font.pixelSize: 11
    }
}
