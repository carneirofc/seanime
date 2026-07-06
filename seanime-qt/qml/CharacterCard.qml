import QtQuick
import QtQuick.Controls

// A small character portrait + name + role, used in the DetailView strip.
Item {
    id: card

    Column {
        anchors.fill: parent
        spacing: 4

        Rectangle {
            width: parent.width
            height: 132
            radius: 6
            clip: true
            color: "#0e0e12"
            Image {
                anchors.fill: parent
                source: imageUrl
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
            }
        }

        Label {
            width: parent.width
            text: name
            color: "#e6e6ee"
            font.pixelSize: 12
            elide: Text.ElideRight
            maximumLineCount: 2
            wrapMode: Text.WordWrap
        }

        Label {
            width: parent.width
            text: role
            color: "#8a8a96"
            font.pixelSize: 10
            elide: Text.ElideRight
            visible: role.length > 0
        }
    }
}
