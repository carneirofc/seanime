import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Confirms the download destination for the selected torrent, then sends it to
// the configured torrent client via the AppController. When the picked torrent
// is a batch of a finished series with missing episodes, also offers a
// "Download missing episodes" (smart-select) action.
Dialog {
    id: dialog
    objectName: "downloadConfirmDialog"
    modal: true
    title: "Download to library"
    anchors.centerIn: Overlay.overlay
    width: 460
    closePolicy: Popup.CloseOnEscape

    // Pre-fill the destination from the controller each time it opens.
    onOpened: destinationField.text = app.torrentDefaultDestination

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

    contentItem: ColumnLayout {
        spacing: 12

        Label {
            Layout.fillWidth: true
            text: app.torrentSelectedName
            color: Theme.text
            font.pixelSize: Theme.fontBase
            font.bold: true
            wrapMode: Text.WordWrap
        }

        Label { text: "Destination"; color: Theme.textDim; font.pixelSize: Theme.fontSm }
        AppTextField {
            id: destinationField
            objectName: "torrentDestinationField"
            Layout.fillWidth: true
            placeholderText: "Absolute path in your library…"
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 8
            AppButton {
                objectName: "torrentDownloadConfirmButton"
                Layout.fillWidth: true
                text: "Download"
                iconName: "download"
                enabled: !app.torrentDownloading && destinationField.text.trim().length > 0
                onClicked: app.startTorrentDownload(destinationField.text, false)
            }
            AppButton {
                objectName: "torrentDownloadMissingButton"
                Layout.fillWidth: true
                visible: app.torrentCanSmartSelect
                text: "Download missing episodes"
                enabled: !app.torrentDownloading && destinationField.text.trim().length > 0
                onClicked: app.startTorrentDownload(destinationField.text, true)
            }
            AppButton {
                objectName: "torrentDownloadCancelButton"
                Layout.fillWidth: true
                text: "Cancel"
                onClicked: dialog.close()
            }
        }
    }
}
