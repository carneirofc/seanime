import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// A titled poster grid over any anime/search model. Extracted so the "split
// adult content" view can stack two of them (a safe grid and an adult grid)
// without duplicating the card layout, keyboard navigation, and transitions.
ColumnLayout {
    id: root

    property alias model: grid.model
    property string title: ""
    // Forwarded when a card is activated (carries the media id).
    signal openRequested(int mediaId)

    spacing: 6
    visible: grid.count > 0

    RowLayout {
        Layout.fillWidth: true
        visible: root.title.length > 0
        spacing: 8
        Label {
            text: root.title
            color: Theme.textStrong
            font.pixelSize: Theme.fontLg
            font.bold: true
        }
        Rectangle {
            Layout.alignment: Qt.AlignVCenter
            height: 20
            width: countLabel.implicitWidth + 12
            radius: Theme.radiusPill
            color: Theme.elevated
            Label {
                id: countLabel
                anchors.centerIn: parent
                text: grid.count
                color: Theme.textDim
                font.pixelSize: Theme.fontXs
            }
        }
    }

    GridView {
        id: grid
        objectName: "mediaGrid"
        Layout.fillWidth: true
        // Height to fit its rows: this grid lives inside a scrolling column, so it
        // must not scroll itself.
        Layout.preferredHeight: {
            var cols = Math.max(1, Math.floor(width / cellWidth))
            var rows = Math.ceil(count / cols)
            return rows * cellHeight
        }
        interactive: false
        cellWidth: 180
        cellHeight: 290

        delegate: AnimeCard {
            width: grid.cellWidth - 12
            height: grid.cellHeight - 12
            onActivated: root.openRequested(mediaId)
        }

        add: Transition {
            NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
        }
    }
}
