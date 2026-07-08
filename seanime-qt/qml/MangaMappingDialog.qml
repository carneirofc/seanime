import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Manual source-match ("mapping") dialog for the open manga. Searches the active
// provider, shows candidate results as cover cards, and maps the AniList entry to
// the chosen provider manga ID via the AppController. Also surfaces the current
// mapping with a "Remove mapping" action. Hosted once at the window level (opened
// on app.mangaMappingOpened, closed on app.mangaMappingSaved).
Dialog {
    id: dialog
    objectName: "mangaMappingDialog"
    modal: true
    title: "Manual match"
    anchors.centerIn: Overlay.overlay
    width: 680
    closePolicy: Popup.CloseOnEscape

    // The candidate the user has picked in the grid (empty = none selected).
    property string selectedId: ""
    property string selectedTitle: ""

    // Reset the transient selection and query each time the dialog opens.
    onOpened: {
        dialog.selectedId = ""
        dialog.selectedTitle = ""
        queryField.text = ""
    }

    background: Rectangle { color: Theme.surface; radius: Theme.radius; border.color: Theme.border }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
            NumberAnimation { property: "scale"; from: 0.96; to: 1; duration: Theme.durBase; easing.type: Theme.easeEmphasis }
        }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Theme.durFast }
    }

    function runSearch() {
        if (queryField.text.trim().length > 0)
            app.runMangaMappingSearch(queryField.text)
    }

    contentItem: ColumnLayout {
        spacing: 12

        // ---- current mapping ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 10
            Label {
                Layout.fillWidth: true
                text: app.mangaMappingCurrent.length > 0
                        ? "Current mapping: " + app.mangaMappingCurrent
                        : "No manual match"
                color: app.mangaMappingCurrent.length > 0 ? Theme.text : Theme.textMuted
                font.pixelSize: Theme.fontBase
                font.italic: app.mangaMappingCurrent.length === 0
                elide: Text.ElideRight
            }
            AppButton {
                objectName: "mangaMappingRemoveButton"
                visible: app.mangaMappingCurrent.length > 0
                text: "Remove mapping"
                iconName: "trash"
                enabled: !app.mangaMappingBusy
                onClicked: app.removeMangaMapping()
            }
        }

        Rectangle { Layout.fillWidth: true; Layout.preferredHeight: 1; color: Theme.border }

        Label {
            Layout.fillWidth: true
            text: "Search the selected provider and choose the correct result."
            color: Theme.textMuted
            font.pixelSize: Theme.fontSm
            horizontalAlignment: Text.AlignHCenter
        }

        // ---- search ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            AppTextField {
                id: queryField
                objectName: "mangaMappingQueryField"
                Layout.fillWidth: true
                placeholderText: "Enter a title…"
                onAccepted: dialog.runSearch()
            }
            AppButton {
                objectName: "mangaMappingSearchButton"
                text: "Search"
                iconName: "search"
                enabled: !app.mangaMappingSearching && queryField.text.trim().length > 0
                onClicked: dialog.runSearch()
            }
        }

        // ---- states ----
        Label {
            Layout.fillWidth: true
            visible: app.mangaMappingSearching
            text: "Searching…"
            color: Theme.textMuted
            font.pixelSize: Theme.fontMd
            horizontalAlignment: Text.AlignHCenter
        }
        Label {
            Layout.fillWidth: true
            visible: !app.mangaMappingSearching && resultsGrid.count === 0
            text: "No results yet. Search above to find a source to match."
            color: Theme.textMuted
            font.pixelSize: Theme.fontMd
            horizontalAlignment: Text.AlignHCenter
        }

        // ---- results ----
        GridView {
            id: resultsGrid
            objectName: "mangaMappingResults"
            Layout.fillWidth: true
            Layout.preferredHeight: 340
            visible: !app.mangaMappingSearching && count > 0
            clip: true
            cellWidth: 128
            cellHeight: 184
            model: app.mangaSearchModel
            ScrollBar.vertical: ScrollBar {}

            delegate: Item {
                width: resultsGrid.cellWidth
                height: resultsGrid.cellHeight

                Rectangle {
                    anchors.fill: parent
                    anchors.margins: 4
                    radius: Theme.radius
                    clip: true
                    color: Theme.surfaceAlt
                    border.width: dialog.selectedId === model.mangaId ? 2 : 1
                    border.color: dialog.selectedId === model.mangaId ? Theme.accent : Theme.border

                    // Cover.
                    Image {
                        anchors.fill: parent
                        source: model.imageUrl
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                        opacity: dialog.selectedId === model.mangaId ? 1.0 : 0.85
                    }

                    // Bottom gradient + labels.
                    Rectangle {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        height: parent.height * 0.6
                        gradient: Gradient {
                            GradientStop { position: 0.0; color: "transparent" }
                            GradientStop { position: 1.0; color: Theme.bg }
                        }
                    }
                    ColumnLayout {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        anchors.margins: 6
                        spacing: 1
                        Label {
                            Layout.fillWidth: true
                            text: model.year > 0 ? model.title + " (" + model.year + ")" : model.title
                            color: Theme.textStrong
                            font.pixelSize: Theme.fontSm
                            font.bold: true
                            wrapMode: Text.WordWrap
                            maximumLineCount: 2
                            elide: Text.ElideRight
                        }
                        Label {
                            Layout.fillWidth: true
                            text: "ID: " + model.mangaId
                            color: Theme.textMuted
                            font.pixelSize: Theme.fontXs
                            elide: Text.ElideRight
                        }
                    }

                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler {
                        onTapped: {
                            dialog.selectedId = model.mangaId
                            dialog.selectedTitle = model.title
                        }
                    }
                }
            }
        }

        Rectangle { Layout.fillWidth: true; Layout.preferredHeight: 1; color: Theme.border }

        // ---- actions ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            Item { Layout.fillWidth: true }
            AppButton {
                objectName: "mangaMappingCancelButton"
                text: "Cancel"
                onClicked: dialog.close()
            }
            AppButton {
                objectName: "mangaMappingConfirmButton"
                text: dialog.selectedId.length > 0 ? "Match to \"" + dialog.selectedTitle + "\"" : "Match"
                iconName: "check"
                enabled: !app.mangaMappingBusy && dialog.selectedId.length > 0
                onClicked: app.confirmMangaMapping(dialog.selectedId)
            }
        }
    }
}
