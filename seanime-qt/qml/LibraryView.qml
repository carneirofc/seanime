import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    signal loginRequested()

    // True when we're connected but the collection failed to load — almost always
    // an expired/invalid AniList token rather than a genuinely empty library.
    readonly property bool loadFailed: app.connectionStatus === "connected"
                                       && app.errorMessage.length > 0

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        // Live "find in library" filter (client-side over the loaded grid).
        TextField {
            id: filterField
            objectName: "libraryFilterField"
            Layout.fillWidth: true
            placeholderText: "Find in library…"
            enabled: grid.count > 0 || text.length > 0
            onTextChanged: app.setLibraryFilter(text)
            Component.onCompleted: app.setLibraryFilter("")  // start unfiltered
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
                color: "#c8c8d0"
                font.pixelSize: 17
                text: app.connectionStatus !== "connected"
                        ? "Not connected. Set host/port and press Connect."
                        : filterField.text.length > 0
                            ? "No matches."
                            : root.loadFailed
                                ? "Couldn't load your library"
                                : "Library is empty."
            }

            // Actionable auth hint when the load failed.
            Label {
                Layout.alignment: Qt.AlignHCenter
                horizontalAlignment: Text.AlignHCenter
                visible: root.loadFailed && filterField.text.length === 0
                color: "#8a8a96"
                font.pixelSize: 13
                text: "Your AniList session may have expired (" + app.errorMessage + ").\n"
                      + "Log in with AniList to load your collection."
            }

            RowLayout {
                Layout.alignment: Qt.AlignHCenter
                visible: root.loadFailed && filterField.text.length === 0
                spacing: 8
                Button {
                    objectName: "libraryLoginButton"
                    text: "Log in with AniList"
                    onClicked: root.loginRequested()
                }
                Button {
                    objectName: "libraryRetryButton"
                    text: "Retry"
                    onClicked: app.refresh()
                }
            }
        }

        GridView {
            id: grid
            objectName: "libraryGrid"
            Layout.fillWidth: true
            Layout.fillHeight: true
            cellWidth: 180
            cellHeight: 290
            clip: true
            model: app.libraryModel

            // Keyboard: reachable via Tab, arrow keys move the selection, and
            // Enter/Return opens the highlighted poster.
            activeFocusOnTab: true
            keyNavigationEnabled: true
            highlightMoveDuration: 100
            highlight: Rectangle {
                radius: 8
                color: "transparent"
                border.width: 2
                border.color: "#3ea6ff"
                visible: grid.activeFocus
            }
            Keys.onReturnPressed: if (grid.currentItem) grid.currentItem.activate()
            Keys.onEnterPressed: if (grid.currentItem) grid.currentItem.activate()

            ScrollBar.vertical: ScrollBar {}

            delegate: AnimeCard {
                width: grid.cellWidth - 12
                height: grid.cellHeight - 12
                onActivated: app.openAnime(mediaId)
            }
        }
    }
}
