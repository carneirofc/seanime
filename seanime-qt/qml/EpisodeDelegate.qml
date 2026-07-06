import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    id: row
    height: 84
    radius: Theme.radius
    color: hover.hovered ? Theme.surfaceHover : Theme.surface
    border.width: 1
    border.color: hover.hovered ? Theme.border : "transparent"

    Behavior on color { ColorAnimation { duration: Theme.durFast } }
    Behavior on border.color { ColorAnimation { duration: Theme.durFast } }

    // Accessibility: announce the row as a labelled group so screen readers read
    // the episode title alongside the watch/download controls it contains.
    Accessible.role: Accessible.ListItem
    Accessible.name: title + (isWatched ? ", watched" : "")

    HoverHandler { id: hover }

    RowLayout {
        anchors.fill: parent
        anchors.margins: 8
        spacing: 12

        Rectangle {
            Layout.preferredWidth: 120
            Layout.preferredHeight: 68
            radius: Theme.radiusSm
            clip: true
            color: Theme.inset
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
                color: Theme.success
                Label {
                    anchors.centerIn: parent
                    text: "✓"; color: Theme.textStrong; font.pixelSize: Theme.fontXs
                }
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2
            Label {
                Layout.fillWidth: true
                text: title
                color: isWatched ? Theme.textMuted : Theme.text
                font.pixelSize: Theme.fontBase
                font.bold: true
                elide: Text.ElideRight
            }
            Label {
                Layout.fillWidth: true
                text: summary
                color: Theme.textMuted
                font.pixelSize: Theme.fontSm
                elide: Text.ElideRight
                maximumLineCount: 2
                wrapMode: Text.WordWrap
                visible: summary.length > 0
            }
        }

        Rectangle {
            Layout.preferredWidth: downloadedLabel.implicitWidth + 16
            Layout.preferredHeight: 22
            radius: Theme.radiusSm
            color: isDownloaded ? Theme.successFill : Theme.elevated
            Label {
                id: downloadedLabel
                anchors.centerIn: parent
                text: isDownloaded ? "Downloaded" : "Not local"
                color: isDownloaded ? Theme.successText : Theme.textDim
                font.pixelSize: Theme.fontXs
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
