import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Manga library: a poster grid over the user's AniList manga collection. Mirrors
// LibraryView; reuses the AnimeCard delegate (its count badge shows chapters).
Item {
    id: root
    signal loginRequested()

    // True when connected but the collection failed to load — almost always an
    // expired/invalid AniList token rather than a genuinely empty library.
    readonly property bool loadFailed: app.connectionStatus === "connected"
                                       && app.errorMessage.length > 0

    // Load the collection when this page is shown.
    Component.onCompleted: app.loadMangaLibrary()

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            Label {
                text: "Manga"
                color: Theme.textStrong
                font.pixelSize: Theme.fontXxl
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            AppButton {
                objectName: "mangaRefreshButton"
                text: "Refresh"
                onClicked: app.loadMangaLibrary()
            }
        }

        // Empty / error state.
        ColumnLayout {
            Layout.fillWidth: true
            Layout.topMargin: 40
            visible: grid.count === 0
            spacing: 10

            Label {
                Layout.alignment: Qt.AlignHCenter
                horizontalAlignment: Text.AlignHCenter
                color: Theme.textDim
                font.pixelSize: Theme.fontXl
                text: app.connectionStatus !== "connected"
                        ? "Not connected. Set host/port and press Connect."
                        : root.loadFailed
                            ? "Couldn't load your manga collection"
                            : "No manga in your collection."
            }

            Label {
                Layout.alignment: Qt.AlignHCenter
                horizontalAlignment: Text.AlignHCenter
                visible: root.loadFailed
                color: Theme.textMuted
                font.pixelSize: Theme.fontMd
                text: "Your AniList session may have expired (" + app.errorMessage + ").\n"
                      + "Log in with AniList to load your collection."
            }

            RowLayout {
                Layout.alignment: Qt.AlignHCenter
                visible: root.loadFailed
                spacing: 8
                AppButton {
                    objectName: "mangaLoginButton"
                    text: "Log in with AniList"
                    onClicked: root.loginRequested()
                }
                AppButton {
                    objectName: "mangaRetryButton"
                    text: "Retry"
                    onClicked: app.loadMangaLibrary()
                }
            }
        }

        // Split view: when the server splits adult content, the collection is
        // shown as separate "Manga" and "Adult" sections. Mirrors LibraryView.
        ScrollView {
            id: splitScroll
            visible: app.splitAdultContent && grid.count > 0
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            ScrollBar.horizontal.policy: ScrollBar.AlwaysOff

            ColumnLayout {
                width: splitScroll.availableWidth
                spacing: 16

                MediaGrid {
                    Layout.fillWidth: true
                    title: "Manga"
                    model: app.mangaLibrarySfwModel
                    onOpenRequested: (mediaId) => app.openManga(mediaId)
                }
                MediaGrid {
                    Layout.fillWidth: true
                    title: "Adult"
                    model: app.mangaLibraryAdultModel
                    onOpenRequested: (mediaId) => app.openManga(mediaId)
                }
            }
        }

        GridView {
            id: grid
            objectName: "mangaGrid"
            visible: !app.splitAdultContent
            Layout.fillWidth: true
            Layout.fillHeight: true
            cellWidth: Theme.posterCellWidth
            cellHeight: Theme.posterCellHeight
            clip: true
            model: app.mangaLibraryModel

            activeFocusOnTab: true
            keyNavigationEnabled: true
            highlightMoveDuration: 100
            highlight: Rectangle {
                radius: Theme.radius
                color: "transparent"
                border.width: 2
                border.color: Theme.accent
                visible: grid.activeFocus
            }
            Keys.onReturnPressed: if (grid.currentItem) grid.currentItem.activate()
            Keys.onEnterPressed: if (grid.currentItem) grid.currentItem.activate()

            ScrollBar.vertical: ScrollBar {}

            // Staggered fade-in as the grid first populates; new items fade too.
            populate: Transition {
                SequentialAnimation {
                    PauseAnimation { duration: Math.max(0, Math.min(ViewTransition.index, 12)) * 22 }
                    NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durSlow; easing.type: Theme.easeStandard }
                }
            }
            add: Transition {
                NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
            }

            delegate: AnimeCard {
                width: grid.cellWidth - 12
                height: grid.cellHeight - 12
                onActivated: app.openManga(mediaId)
            }
        }
    }
}
