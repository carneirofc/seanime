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
        color: Theme.textStrong
        font.pixelSize: Theme.fontLg
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
            radius: Theme.radius
            color: "transparent"
            border.width: 2
            border.color: Theme.accent
            visible: row.activeFocus
        }
        Keys.onReturnPressed: if (row.currentItem) row.currentItem.activate()
        Keys.onEnterPressed: if (row.currentItem) row.currentItem.activate()

        // Staggered fade-in as the carousel populates.
        populate: Transition {
            SequentialAnimation {
                PauseAnimation { duration: Math.max(0, Math.min(ViewTransition.index, 10)) * 25 }
                NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durSlow; easing.type: Theme.easeStandard }
            }
        }
        add: Transition {
            NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
        }

        ScrollBar.horizontal: ScrollBar { policy: ScrollBar.AsNeeded }

        delegate: AnimeCard {
            width: 168
            height: 272
            onActivated: app.openAnime(mediaId)
        }
    }
}
