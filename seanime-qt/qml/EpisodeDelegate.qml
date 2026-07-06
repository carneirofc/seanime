import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    id: row
    height: 84
    radius: 6
    color: "#1a1a22"

    // Accessibility: announce the row as a labelled group so screen readers read
    // the episode title alongside the watch/download controls it contains.
    Accessible.role: Accessible.ListItem
    Accessible.name: title + (isWatched ? ", watched" : "")

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
            // Watched overlay tick.
            Rectangle {
                visible: isWatched
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                anchors.margins: 3
                width: 18; height: 18; radius: 9
                color: "#1f7a3a"
                Label {
                    anchors.centerIn: parent
                    text: "✓"; color: "#ffffff"; font.pixelSize: 11
                }
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2
            Label {
                Layout.fillWidth: true
                text: title
                color: isWatched ? "#9a9aa6" : "#e6e6ee"
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

        // Mark watched up to this episode / unwatch (set progress to the one before).
        Button {
            objectName: "episodeWatchButton_" + progressNumber
            text: isWatched ? "Unwatch" : "Mark watched"
            onClicked: app.setEpisodeProgress(isWatched ? progressNumber - 1 : progressNumber)
        }
    }
}
