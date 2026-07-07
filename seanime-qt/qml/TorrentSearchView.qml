import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Torrent browser for the open library anime. Smart-searches the Seanime server
// (provider left to the server default) and lists results; tapping a result asks
// the AppController to open the download confirm dialog.
Item {
    id: root
    signal back()

    readonly property var resolutionOptions: [
        { label: "Any resolution", value: "" },
        { label: "1080p", value: "1080" },
        { label: "720p", value: "720" },
        { label: "540p", value: "540" },
        { label: "480p", value: "480" }
    ]

    function runSearch() {
        app.runTorrentSearch(
            queryField.text,
            parseInt(episodeField.text) || 0,
            batchSwitch.checked,
            resolutionCombo.currentValue)
    }

    // Seed the controls from the search context the controller set when opening.
    Component.onCompleted: {
        batchSwitch.checked = app.torrentSearchBatch
        if (app.torrentSearchEpisode > 0)
            episodeField.text = app.torrentSearchEpisode
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        // Top bar: back + title.
        RowLayout {
            Layout.fillWidth: true
            spacing: 10
            AppButton {
                objectName: "torrentBackButton"
                iconName: "arrow-left"
                text: "Back"
                onClicked: root.back()
            }
            Label {
                Layout.fillWidth: true
                text: "Download · " + app.detailTitle
                color: Theme.textStrong
                font.pixelSize: Theme.fontXl
                font.bold: true
                elide: Text.ElideRight
            }
        }

        // Search controls.
        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            AppTextField {
                id: queryField
                objectName: "torrentQueryField"
                Layout.fillWidth: true
                placeholderText: "Refine query (optional)…"
                onAccepted: root.runSearch()
            }
            AppComboBox {
                id: resolutionCombo
                objectName: "torrentResolutionCombo"
                width: 150
                textRole: "label"
                valueRole: "value"
                model: root.resolutionOptions
            }
            AppTextField {
                id: episodeField
                objectName: "torrentEpisodeField"
                width: 90
                placeholderText: "Episode"
                inputMethodHints: Qt.ImhDigitsOnly
                validator: IntValidator { bottom: 0; top: 9999 }
                onAccepted: root.runSearch()
            }
            AppSwitch {
                id: batchSwitch
                objectName: "torrentBatchSwitch"
                text: "Batch"
            }
            AppButton {
                objectName: "torrentSearchButton"
                text: "Search"
                onClicked: root.runSearch()
            }
        }

        // Loading + empty states.
        Label {
            Layout.fillWidth: true
            visible: app.torrentSearchLoading
            text: "Searching torrents…"
            color: Theme.textMuted
            font.pixelSize: Theme.fontLg
            horizontalAlignment: Text.AlignHCenter
        }
        Label {
            Layout.fillWidth: true
            visible: !app.torrentSearchLoading && list.count === 0
            text: "No torrents found. Try a different query or filters."
            color: Theme.textMuted
            font.pixelSize: Theme.fontLg
            horizontalAlignment: Text.AlignHCenter
        }

        ListView {
            id: list
            objectName: "torrentList"
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            spacing: 8
            model: app.torrentModel
            ScrollBar.vertical: ScrollBar {}
            delegate: TorrentDelegate {
                width: list.width
                onSelected: app.selectTorrent(index)
            }
        }
    }
}
