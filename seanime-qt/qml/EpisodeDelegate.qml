import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    id: row
    height: 84
    radius: 6
    color: "#1a1a22"

    RowLayout {
        anchors.fill: parent
        anchors.margins: 8
        spacing: 12

        Rectangle {
            Layout.preferredWidth: 120
            Layout.preferredHeight: 68
            radius: 4
            clip: true
            color: "#0e0e12"
            Image {
                anchors.fill: parent
                source: thumbnailUrl
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2
            Label {
                Layout.fillWidth: true
                text: title
                color: "#e6e6ee"
                font.pixelSize: 14
                font.bold: true
                elide: Text.ElideRight
            }
            Label {
                Layout.fillWidth: true
                text: summary
                color: "#9a9aa6"
                font.pixelSize: 12
                elide: Text.ElideRight
                maximumLineCount: 2
                wrapMode: Text.WordWrap
                visible: summary.length > 0
            }
        }

        Rectangle {
            Layout.preferredWidth: downloadedLabel.implicitWidth + 16
            Layout.preferredHeight: 22
            radius: 4
            color: isDownloaded ? "#1f4a2a" : "#3a3a48"
            Label {
                id: downloadedLabel
                anchors.centerIn: parent
                text: isDownloaded ? "Downloaded" : "Not local"
                color: isDownloaded ? "#8fe6a3" : "#c8c8d0"
                font.pixelSize: 11
            }
        }
    }
}
