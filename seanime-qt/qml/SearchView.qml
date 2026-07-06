import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// AniList advanced search: title + filters (sort, genres, format, season, year,
// status, min score) posted to the backend's list-anime endpoint. Results reuse
// the AnimeCard delegate and open in the detail view.
Item {
    id: root

    readonly property var sortOptions: [
        { label: "Sort: Default", value: "" },
        { label: "Trending", value: "TRENDING_DESC" },
        { label: "Score", value: "SCORE_DESC" },
        { label: "Popularity", value: "POPULARITY_DESC" },
        { label: "Newest", value: "START_DATE_DESC" },
        { label: "Favorites", value: "FAVOURITES_DESC" }
    ]
    readonly property var formatOptions: [
        { label: "Any format", value: "" },
        { label: "TV", value: "TV" },
        { label: "TV Short", value: "TV_SHORT" },
        { label: "Movie", value: "MOVIE" },
        { label: "Special", value: "SPECIAL" },
        { label: "OVA", value: "OVA" },
        { label: "ONA", value: "ONA" }
    ]
    readonly property var seasonOptions: [
        { label: "Any season", value: "" },
        { label: "Winter", value: "WINTER" },
        { label: "Spring", value: "SPRING" },
        { label: "Summer", value: "SUMMER" },
        { label: "Fall", value: "FALL" }
    ]
    readonly property var statusOptions: [
        { label: "Any status", value: "" },
        { label: "Releasing", value: "RELEASING" },
        { label: "Finished", value: "FINISHED" },
        { label: "Upcoming", value: "NOT_YET_RELEASED" },
        { label: "Cancelled", value: "CANCELLED" },
        { label: "Hiatus", value: "HIATUS" }
    ]
    function runSearch() {
        app.searchAdvanced({
            "search": searchField.text,
            "sort": sortCombo.currentValue,
            "genres": genrePopup.selected,
            "format": formatCombo.currentValue,
            "season": seasonCombo.currentValue,
            "year": parseInt(yearField.text) || 0,
            "status": statusCombo.currentValue,
            "minScore": minScoreSpin.value
        })
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            TextField {
                id: searchField
                objectName: "searchField"
                Layout.fillWidth: true
                placeholderText: "Search AniList…"
                onAccepted: root.runSearch()
                focus: true
            }
            Button {
                objectName: "searchButton"
                text: "Search"
                onClicked: root.runSearch()
            }
        }

        // Filter bar (wraps on narrow windows).
        Flow {
            Layout.fillWidth: true
            spacing: 8

            ComboBox {
                id: sortCombo
                objectName: "sortCombo"
                width: 150
                textRole: "label"
                valueRole: "value"
                model: root.sortOptions
            }
            ComboBox {
                id: formatCombo
                objectName: "formatCombo"
                width: 130
                textRole: "label"
                valueRole: "value"
                model: root.formatOptions
            }
            ComboBox {
                id: seasonCombo
                objectName: "seasonCombo"
                width: 130
                textRole: "label"
                valueRole: "value"
                model: root.seasonOptions
            }
            ComboBox {
                id: statusCombo
                objectName: "statusCombo"
                width: 140
                textRole: "label"
                valueRole: "value"
                model: root.statusOptions
            }
            TextField {
                id: yearField
                objectName: "yearField"
                width: 90
                placeholderText: "Year"
                inputMethodHints: Qt.ImhDigitsOnly
                validator: IntValidator { bottom: 1940; top: 2100 }
            }
            RowLayout {
                spacing: 4
                Label { text: "Min score"; color: "#8a8a96"; font.pixelSize: 12 }
                SpinBox {
                    id: minScoreSpin
                    objectName: "minScoreSpin"
                    from: 0; to: 100; stepSize: 5
                    Layout.preferredWidth: 110
                }
            }
            Button {
                objectName: "genresButton"
                text: genrePopup.selected.length > 0
                      ? "Genres (" + genrePopup.selected.length + ")"
                      : "Genres"
                onClicked: genrePopup.open()
            }
        }

        Label {
            Layout.fillWidth: true
            visible: grid.count === 0
            text: searchField.text.length === 0 && genrePopup.selected.length === 0
                  ? "Enter a title or pick filters, then press Search."
                  : "No results."
            color: "#8a8a96"
            font.pixelSize: 16
            horizontalAlignment: Text.AlignHCenter
        }

        GridView {
            id: grid
            objectName: "searchGrid"
            Layout.fillWidth: true
            Layout.fillHeight: true
            cellWidth: 180
            cellHeight: 290
            clip: true
            model: app.searchModel

            // Keyboard: reachable via Tab (the search field keeps initial focus);
            // arrow keys move the selection, Enter/Return opens the current card.
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

            footer: Item {
                width: grid.width
                height: grid.count > 0 ? 56 : 0
                Button {
                    objectName: "loadMoreButton"
                    anchors.centerIn: parent
                    text: "Load more"
                    onClicked: app.searchLoadMore()
                }
            }
        }
    }

    // Genre multi-select (owns its selection; read via genrePopup.selected).
    GenrePopup { id: genrePopup }
}
