import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// A titled horizontal strip of AnimeCards bound to a SearchModel-shaped model.
// Reused by DiscoverView (each feed) and DetailView (relations/recommendations).
ColumnLayout {
    id: root
    property string title: ""
    property alias model: row.model
    property alias count: row.count

    spacing: 6
    visible: row.count > 0
    Layout.fillWidth: true

    Label {
        Layout.leftMargin: 4
        text: root.title
        color: "#ffffff"
        font.pixelSize: 16
        font.bold: true
    }

    ListView {
        id: row
        Layout.fillWidth: true
        Layout.preferredHeight: 284
        orientation: ListView.Horizontal
        spacing: 12
        clip: true

        // Keyboard: Tab reaches the row; Left/Right move the selection and
        // Enter/Return opens the highlighted card.
        activeFocusOnTab: true
        keyNavigationEnabled: true
        highlightMoveDuration: 100
        highlight: Rectangle {
            radius: 8
            color: "transparent"
            border.width: 2
            border.color: "#3ea6ff"
            visible: row.activeFocus
        }
        Keys.onReturnPressed: if (row.currentItem) row.currentItem.activate()
        Keys.onEnterPressed: if (row.currentItem) row.currentItem.activate()

        ScrollBar.horizontal: ScrollBar { policy: ScrollBar.AsNeeded }

        delegate: AnimeCard {
            width: 168
            height: 272
            onActivated: app.openAnime(mediaId)
        }
    }
}
