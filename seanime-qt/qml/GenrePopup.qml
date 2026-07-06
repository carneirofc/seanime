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
    background: Rectangle { color: Theme.surface; radius: Theme.radius; border.color: Theme.border }

    // Fade + subtle scale on open/close.
    enter: Transition {
        ParallelAnimation {
            NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
            NumberAnimation { property: "scale"; from: 0.96; to: 1; duration: Theme.durBase; easing.type: Theme.easeEmphasis }
        }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Theme.durFast }
    }

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
        Label { text: "Genres"; color: Theme.textStrong; font.pixelSize: Theme.fontLg; font.bold: true }
        Flow {
            Layout.preferredWidth: 388
            spacing: 6
            Repeater {
                model: genrePopup.genreList
                delegate: AppButton {
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
            AppButton { text: "Clear"; onClicked: genrePopup.selected = [] }
            Item { Layout.fillWidth: true }
            AppButton { text: "Done"; onClicked: genrePopup.close() }
        }
    }
}
