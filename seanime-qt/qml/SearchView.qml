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
            "tags": tagPopup.selected,
            "format": formatCombo.currentValue,
            "season": seasonCombo.currentValue,
            "year": parseInt(yearField.text) || 0,
            "status": statusCombo.currentValue,
            "minScore": minScoreSpin.value,
            // Only ask for adult media when the server allows it and the user opts in.
            "isAdult": app.enableAdultContent && adultSwitch.checked
        })
    }

    // True once any filter is set, so the empty-state copy can adapt.
    readonly property bool hasFilters: searchField.text.length > 0
                                       || genrePopup.selected.length > 0
                                       || tagPopup.selected.length > 0
                                       || (app.enableAdultContent && adultSwitch.checked)

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            AppTextField {
                id: searchField
                objectName: "searchField"
                Layout.fillWidth: true
                placeholderText: "Search AniList…"
                onAccepted: root.runSearch()
                focus: true
            }
            AppButton {
                objectName: "searchButton"
                text: "Search"
                onClicked: root.runSearch()
            }
        }

        // Filter bar (wraps on narrow windows).
        Flow {
            Layout.fillWidth: true
            spacing: 8

            AppComboBox {
                id: sortCombo
                objectName: "sortCombo"
                width: 150
                textRole: "label"
                valueRole: "value"
                model: root.sortOptions
            }
            AppComboBox {
                id: formatCombo
                objectName: "formatCombo"
                width: 130
                textRole: "label"
                valueRole: "value"
                model: root.formatOptions
            }
            AppComboBox {
                id: seasonCombo
                objectName: "seasonCombo"
                width: 130
                textRole: "label"
                valueRole: "value"
                model: root.seasonOptions
            }
            AppComboBox {
                id: statusCombo
                objectName: "statusCombo"
                width: 140
                textRole: "label"
                valueRole: "value"
                model: root.statusOptions
            }
            AppTextField {
                id: yearField
                objectName: "yearField"
                width: 90
                placeholderText: "Year"
                inputMethodHints: Qt.ImhDigitsOnly
                validator: IntValidator { bottom: 1940; top: 2100 }
            }
            RowLayout {
                spacing: 4
                Label { text: "Min score"; color: Theme.textMuted; font.pixelSize: Theme.fontSm }
                AppSpinBox {
                    id: minScoreSpin
                    objectName: "minScoreSpin"
                    from: 0; to: 100; stepSize: 5
                    Layout.preferredWidth: 110
                }
            }
            AppButton {
                objectName: "genresButton"
                text: genrePopup.selected.length > 0
                      ? "Genres (" + genrePopup.selected.length + ")"
                      : "Genres"
                onClicked: genrePopup.open()
            }
            AppButton {
                objectName: "tagsButton"
                text: tagPopup.selected.length > 0
                      ? "Tags (" + tagPopup.selected.length + ")"
                      : "Tags"
                onClicked: tagPopup.open()
            }
            // Adult toggle — only offered when the server enables adult content.
            RowLayout {
                visible: app.enableAdultContent
                spacing: 4
                AppSwitch {
                    id: adultSwitch
                    objectName: "adultSwitch"
                    text: "Adult only"
                    onToggled: root.runSearch()
                }
            }
        }

        Label {
            Layout.fillWidth: true
            visible: app.searchModel.count === 0
            text: !root.hasFilters
                  ? "Enter a title or pick filters, then press Search."
                  : "No results."
            color: Theme.textMuted
            font.pixelSize: Theme.fontLg
            horizontalAlignment: Text.AlignHCenter
        }

        // Split view: when the server splits adult content, results are shown in
        // two labeled sections (safe + adult) that scroll together.
        ScrollView {
            id: splitScroll
            visible: app.splitAdultContent && app.searchModel.count > 0
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            ScrollBar.horizontal.policy: ScrollBar.AlwaysOff

            ColumnLayout {
                width: splitScroll.availableWidth
                spacing: 16

                MediaGrid {
                    Layout.fillWidth: true
                    title: "Results"
                    model: app.searchSfwModel
                    onOpenRequested: (mediaId) => app.openAnime(mediaId)
                }
                MediaGrid {
                    Layout.fillWidth: true
                    title: "Adult"
                    model: app.searchAdultModel
                    onOpenRequested: (mediaId) => app.openAnime(mediaId)
                }
                AppButton {
                    objectName: "loadMoreButtonSplit"
                    Layout.alignment: Qt.AlignHCenter
                    text: "Load more"
                    onClicked: app.searchLoadMore()
                }
            }
        }

        GridView {
            id: grid
            objectName: "searchGrid"
            visible: !app.splitAdultContent
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
                radius: Theme.radius
                color: "transparent"
                border.width: 2
                border.color: Theme.accent
                visible: grid.activeFocus
            }
            Keys.onReturnPressed: if (grid.currentItem) grid.currentItem.activate()
            Keys.onEnterPressed: if (grid.currentItem) grid.currentItem.activate()

            ScrollBar.vertical: ScrollBar {}

            // Staggered fade-in as results populate; "Load more" items fade in too.
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
                onActivated: app.openAnime(mediaId)
            }

            footer: Item {
                width: grid.width
                height: grid.count > 0 ? 56 : 0
                AppButton {
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

    // Tag multi-select (owns its selection; read via tagPopup.selected).
    TagPopup { id: tagPopup }
}
