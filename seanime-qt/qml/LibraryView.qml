import QtQuick
import QtQuick.Controls

Item {
    id: root

    // Empty state when the library has no entries yet.
    Label {
        anchors.centerIn: parent
        visible: grid.count === 0
        text: app.connectionStatus === "connected"
              ? "Library is empty."
              : "Not connected. Set host/port and press Connect."
        color: "#8a8a96"
        font.pixelSize: 16
    }

    GridView {
        id: grid
        anchors.fill: parent
        anchors.margins: 16
        cellWidth: 180
        cellHeight: 290
        clip: true
        model: app.libraryModel

        ScrollBar.vertical: ScrollBar {}

        delegate: AnimeCard {
            width: grid.cellWidth - 12
            height: grid.cellHeight - 12
            onActivated: app.openAnime(mediaId)
        }
    }
}
