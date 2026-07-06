import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Genre multi-select popup. Owns the genre list and the current selection; the
// parent reads `selected` (e.g. in its search query and button label).
Popup {
    id: genrePopup
    objectName: "genrePopup"
    modal: true
    anchors.centerIn: Overlay.overlay
    width: 420
    padding: 16
    background: Rectangle { color: "#1a1a22"; radius: 8; border.color: "#2c2c38" }

    // Currently-selected genres.
    property var selected: []

    readonly property var genreList: [
        "Action", "Adventure", "Comedy", "Drama", "Ecchi", "Fantasy", "Horror",
        "Mahou Shoujo", "Mecha", "Music", "Mystery", "Psychological", "Romance",
        "Sci-Fi", "Slice of Life", "Sports", "Supernatural", "Thriller"
    ]

    function toggle(g) {
        var list = selected.slice()
        var i = list.indexOf(g)
        if (i >= 0) list.splice(i, 1)
        else list.push(g)
        selected = list
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 12
        Label { text: "Genres"; color: "#ffffff"; font.pixelSize: 15; font.bold: true }
        Flow {
            Layout.preferredWidth: 388
            spacing: 6
            Repeater {
                model: genrePopup.genreList
                delegate: Button {
                    required property string modelData
                    text: modelData
                    checkable: true
                    checked: genrePopup.selected.indexOf(modelData) >= 0
                    onToggled: genrePopup.toggle(modelData)
                }
            }
        }
        RowLayout {
            Layout.fillWidth: true
            Button { text: "Clear"; onClicked: genrePopup.selected = [] }
            Item { Layout.fillWidth: true }
            Button { text: "Done"; onClicked: genrePopup.close() }
        }
    }
}
