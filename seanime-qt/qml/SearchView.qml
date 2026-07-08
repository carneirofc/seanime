import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// AniList advanced search: a title field plus a grouped filter panel (sort,
// genres, tags, format, season, year, status, min score, adult). Filters
// auto-run — changing any control re-queries after a short debounce so the
// results stay in sync without a manual "Search" press. Results reuse the
// AnimeCard delegate and open in the detail view.
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

    // Fire the query now, cancelling any pending debounce.
    function runSearch() {
        searchDebounce.stop()
        app.searchAdvanced({
            "search": searchField.text,
            "sort": sortCombo.currentValue,
            "genres": genrePopup.selected,
            "tags": tagPopup.selected,
            "format": formatCombo.currentValue,
            "season": seasonCombo.currentValue,
            // Only treat a full four-digit year as a filter (avoids querying on
            // half-typed input like "20").
            "year": yearField.text.length === 4 ? (parseInt(yearField.text) || 0) : 0,
            "status": statusCombo.currentValue,
            "minScore": minScoreSpin.value,
            // Only ask for adult media when the server allows it and the user opts in.
            "isAdult": app.enableAdultContent && adultSwitch.checked
        })
    }

    // Coalesce rapid filter changes (typing, toggling several genres) into one
    // query instead of firing on every keystroke/click.
    function scheduleSearch() { searchDebounce.restart() }

    // Reset every control to its default and clear the grid.
    function clearFilters() {
        searchField.clear()
        sortCombo.currentIndex = 0
        formatCombo.currentIndex = 0
        seasonCombo.currentIndex = 0
        statusCombo.currentIndex = 0
        yearField.clear()
        minScoreSpin.value = 0
        genrePopup.selected = []
        tagPopup.selected = []
        adultSwitch.checked = false
        root.runSearch()
    }

    // True once any control is off its default — drives the "Clear" button.
    readonly property bool hasFilters: hasQuery || sortCombo.currentIndex > 0

    // True when a filter that actually drives results is set (mirrors the
    // backend's "meaningful" check — sort alone yields nothing). Drives the
    // empty-state copy so "No results" only shows after a real query.
    readonly property bool hasQuery: searchField.text.length > 0
                                     || genrePopup.selected.length > 0
                                     || tagPopup.selected.length > 0
                                     || formatCombo.currentIndex > 0
                                     || seasonCombo.currentIndex > 0
                                     || statusCombo.currentIndex > 0
                                     || yearField.text.length === 4
                                     || minScoreSpin.value > 0
                                     || (app.enableAdultContent && adultSwitch.checked)

    Timer {
        id: searchDebounce
        interval: 280
        onTriggered: root.runSearch()
    }

    // Seed from a genre/tag deep-link (a tapped chip on the detail header):
    // select it and run the search immediately.
    Component.onCompleted: {
        var genre = app.consumePendingSearchGenre()
        var tag = app.consumePendingSearchTag()
        var seeded = false
        if (genre && genre.length > 0) { genrePopup.selected = [genre]; seeded = true }
        if (tag && tag.length > 0) { tagPopup.selected = [tag]; seeded = true }
        if (seeded)
            root.runSearch()
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Theme.spacingLg
        spacing: Theme.spacing

        // ---- title + result count -------------------------------------------
        RowLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingSm
            Icon {
                name: "search"
                size: Theme.fontXl
                color: Theme.accentSoft
                Layout.alignment: Qt.AlignVCenter
            }
            Label {
                text: "Search"
                color: Theme.textStrong
                font.pixelSize: Theme.fontXl
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            Rectangle {
                visible: app.searchModel.count > 0
                Layout.alignment: Qt.AlignVCenter
                implicitWidth: countLabel.implicitWidth + 16
                implicitHeight: 22
                radius: Theme.radiusPill
                color: Theme.elevated
                Label {
                    id: countLabel
                    anchors.centerIn: parent
                    text: app.searchModel.count + " result" + (app.searchModel.count === 1 ? "" : "s")
                    color: Theme.textDim
                    font.pixelSize: Theme.fontXs
                }
            }
        }

        // ---- search bar -----------------------------------------------------
        RowLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingSm

            // Field wrapper hosts the inline leading magnifier and trailing clear (×).
            Item {
                Layout.fillWidth: true
                implicitHeight: Theme.controlHeight

                AppTextField {
                    id: searchField
                    objectName: "searchField"
                    anchors.fill: parent
                    leftPadding: 34
                    rightPadding: clearButton.visible ? 34 : Theme.controlPadding
                    placeholderText: "Search AniList by title…"
                    focus: true
                    onTextChanged: root.scheduleSearch()
                    onAccepted: root.runSearch()
                }
                Icon {
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.left: parent.left
                    anchors.leftMargin: 10
                    name: "search"
                    size: Theme.fontMd
                    color: Theme.textMuted
                }
                Icon {
                    id: clearButton
                    objectName: "searchClearButton"
                    visible: searchField.text.length > 0
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.right: parent.right
                    anchors.rightMargin: 10
                    name: "x"
                    size: Theme.fontMd
                    color: clearHover.hovered ? Theme.text : Theme.textMuted
                    HoverHandler { id: clearHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: { searchField.clear(); searchField.forceActiveFocus() } }
                }
            }

            AppButton {
                objectName: "searchButton"
                text: "Search"
                iconName: "search"
                onClicked: root.runSearch()
            }
        }

        // ---- filter panel ---------------------------------------------------
        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: filterCol.implicitHeight + 2 * Theme.spacing
            radius: Theme.radius
            color: Theme.surfaceAlt
            border.width: 1
            border.color: Theme.border

            ColumnLayout {
                id: filterCol
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: Theme.spacing
                spacing: Theme.spacingSm

                // The control bar wraps on narrow windows.
                Flow {
                    Layout.fillWidth: true
                    spacing: Theme.spacingSm

                    AppComboBox {
                        id: sortCombo
                        objectName: "sortCombo"
                        width: 150
                        textRole: "label"
                        valueRole: "value"
                        model: root.sortOptions
                        onActivated: root.scheduleSearch()
                    }
                    AppComboBox {
                        id: formatCombo
                        objectName: "formatCombo"
                        width: 130
                        textRole: "label"
                        valueRole: "value"
                        model: root.formatOptions
                        onActivated: root.scheduleSearch()
                    }
                    AppComboBox {
                        id: seasonCombo
                        objectName: "seasonCombo"
                        width: 130
                        textRole: "label"
                        valueRole: "value"
                        model: root.seasonOptions
                        onActivated: root.scheduleSearch()
                    }
                    AppComboBox {
                        id: statusCombo
                        objectName: "statusCombo"
                        width: 140
                        textRole: "label"
                        valueRole: "value"
                        model: root.statusOptions
                        onActivated: root.scheduleSearch()
                    }
                    AppTextField {
                        id: yearField
                        objectName: "yearField"
                        width: 90
                        placeholderText: "Year"
                        inputMethodHints: Qt.ImhDigitsOnly
                        validator: IntValidator { bottom: 1940; top: 2100 }
                        onTextChanged: root.scheduleSearch()
                    }
                    // Min-score stepper with its label, kept together as a unit.
                    Row {
                        spacing: Theme.spacingXs
                        height: Theme.controlHeight
                        Label {
                            text: "Min score"
                            color: Theme.textMuted
                            font.pixelSize: Theme.fontSm
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        AppSpinBox {
                            id: minScoreSpin
                            objectName: "minScoreSpin"
                            width: 110
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0; to: 100; stepSize: 5
                            onValueModified: root.scheduleSearch()
                        }
                    }
                    AppButton {
                        objectName: "genresButton"
                        iconName: "adjustments-horizontal"
                        text: genrePopup.selected.length > 0
                              ? "Genres (" + genrePopup.selected.length + ")"
                              : "Genres"
                        onClicked: genrePopup.open()
                    }
                    AppButton {
                        objectName: "tagsButton"
                        iconName: "adjustments-horizontal"
                        text: tagPopup.selected.length > 0
                              ? "Tags (" + tagPopup.selected.length + ")"
                              : "Tags"
                        onClicked: tagPopup.open()
                    }
                    // Adult toggle — only offered when the server enables adult content.
                    Row {
                        visible: app.enableAdultContent
                        height: Theme.controlHeight
                        AppSwitch {
                            id: adultSwitch
                            objectName: "adultSwitch"
                            text: "Adult only"
                            anchors.verticalCenter: parent.verticalCenter
                            onToggled: root.scheduleSearch()
                        }
                    }
                    AppButton {
                        objectName: "clearFiltersButton"
                        visible: root.hasFilters
                        iconName: "x"
                        text: "Clear"
                        onClicked: root.clearFilters()
                    }
                }

                // Active genre/tag selections as removable chips.
                Flow {
                    Layout.fillWidth: true
                    spacing: Theme.spacingXs
                    visible: genrePopup.selected.length > 0 || tagPopup.selected.length > 0

                    Repeater {
                        model: genrePopup.selected
                        delegate: Chip {
                            required property string modelData
                            icon: "x"
                            text: modelData
                            interactive: true
                            textColor: Theme.textDim
                            fillColor: Theme.elevated
                            onClicked: { genrePopup.toggle(modelData); root.scheduleSearch() }
                        }
                    }
                    Repeater {
                        model: tagPopup.selected
                        delegate: Chip {
                            required property string modelData
                            icon: "x"
                            text: modelData
                            interactive: true
                            textColor: Theme.accentSoft
                            fillColor: Theme.accentFill
                            onClicked: { tagPopup.toggle(modelData); root.scheduleSearch() }
                        }
                    }
                }
            }
        }

        // ---- results --------------------------------------------------------
        // Everything shares one fill-height container and toggles by visibility,
        // so the empty state and the grid never fight for layout space.
        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            // Busy: an initial search is in flight and there is nothing to show yet.
            ColumnLayout {
                anchors.centerIn: parent
                visible: app.searchBusy && app.searchModel.count === 0
                spacing: Theme.spacing
                Row {
                    Layout.alignment: Qt.AlignHCenter
                    spacing: 6
                    Repeater {
                        model: 3
                        delegate: Rectangle {
                            required property int index
                            width: 10; height: 10; radius: 5
                            color: Theme.accent
                            SequentialAnimation on opacity {
                                loops: Animation.Infinite
                                PauseAnimation { duration: index * 140 }
                                NumberAnimation { from: 0.3; to: 1.0; duration: 320; easing.type: Theme.easeInOut }
                                NumberAnimation { from: 1.0; to: 0.3; duration: 320; easing.type: Theme.easeInOut }
                                PauseAnimation { duration: (2 - index) * 140 }
                            }
                        }
                    }
                }
                Label {
                    Layout.alignment: Qt.AlignHCenter
                    text: "Searching…"
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontMd
                }
            }

            // Empty: no search running and no results.
            ColumnLayout {
                anchors.centerIn: parent
                width: Math.min(parent.width - 32, 420)
                visible: !app.searchBusy && app.searchModel.count === 0
                spacing: Theme.spacingSm
                Icon {
                    Layout.alignment: Qt.AlignHCenter
                    name: "search"
                    size: 48
                    color: Theme.borderStrong
                }
                Label {
                    Layout.alignment: Qt.AlignHCenter
                    text: root.hasQuery ? "No results" : "Search AniList"
                    color: Theme.textDim
                    font.pixelSize: Theme.fontLg
                    font.bold: true
                }
                Label {
                    Layout.fillWidth: true
                    text: root.hasQuery
                          ? "Nothing matched those filters. Try a different title or loosen the filters."
                          : "Type a title above, or pick filters to explore the catalog."
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontMd
                    wrapMode: Text.WordWrap
                    horizontalAlignment: Text.AlignHCenter
                }
            }

            // Split view: when the server splits adult content, results show in
            // two labeled sections (safe + adult) that scroll together.
            ScrollView {
                id: splitScroll
                anchors.fill: parent
                visible: app.splitAdultContent && app.searchModel.count > 0
                clip: true
                ScrollBar.horizontal.policy: ScrollBar.AlwaysOff

                ColumnLayout {
                    width: splitScroll.availableWidth
                    spacing: Theme.spacingLg

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
                        text: app.searchBusy ? "Loading…" : "Load more"
                        enabled: !app.searchBusy
                        onClicked: app.searchLoadMore()
                    }
                }
            }

            // Single grid (the common case).
            GridView {
                id: grid
                objectName: "searchGrid"
                anchors.fill: parent
                visible: !app.splitAdultContent && app.searchModel.count > 0
                cellWidth: Theme.posterCellWidth
                cellHeight: Theme.posterCellHeight
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

                // A single light fade as results populate — no per-item stagger, so
                // a full page appears smoothly without a burst of layout work.
                populate: Transition {
                    NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
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
                    height: 56
                    AppButton {
                        objectName: "loadMoreButton"
                        anchors.centerIn: parent
                        text: app.searchBusy ? "Loading…" : "Load more"
                        enabled: !app.searchBusy
                        onClicked: app.searchLoadMore()
                    }
                }
            }
        }
    }

    // Genre multi-select (owns its selection; read via genrePopup.selected).
    // Re-runs the search when closed so popup picks take effect.
    GenrePopup { id: genrePopup; onClosed: root.scheduleSearch() }

    // Tag multi-select (owns its selection; read via tagPopup.selected).
    TagPopup { id: tagPopup; onClosed: root.scheduleSearch() }
}
