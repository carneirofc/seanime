import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Modal editor for the user's AniList list entry (status / score / progress).
// Pre-fills from the current app.detailList* state and persists via
// app.saveListEntry(status, score, progress).
Dialog {
    id: editor
    objectName: "listEntryEditor"
    modal: true
    title: "Edit list entry"
    anchors.centerIn: Overlay.overlay
    width: 340
    standardButtons: Dialog.Save | Dialog.Cancel

    property int episodeCount: 0

    // AniList MediaListStatus values with friendly labels.
    readonly property var statuses: [
        { label: "Watching", value: "CURRENT" },
        { label: "Planning", value: "PLANNING" },
        { label: "Completed", value: "COMPLETED" },
        { label: "Paused", value: "PAUSED" },
        { label: "Dropped", value: "DROPPED" },
        { label: "Rewatching", value: "REPEATING" }
    ]

    function openFor() {
        var idx = 0
        for (var i = 0; i < statuses.length; i++)
            if (statuses[i].value === app.detailListStatus) idx = i
        statusCombo.currentIndex = idx
        scoreSpin.value = app.detailListScore
        progressSpin.value = app.detailListProgress
        open()
    }

    onAccepted: app.saveListEntry(
        statuses[statusCombo.currentIndex].value,
        scoreSpin.value,
        progressSpin.value)

    background: Rectangle { color: "#1a1a22"; radius: 8; border.color: "#2c2c38" }

    contentItem: ColumnLayout {
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            Label { text: "Status"; color: "#c0c0cc"; Layout.preferredWidth: 90 }
            ComboBox {
                id: statusCombo
                objectName: "listStatusCombo"
                Layout.fillWidth: true
                textRole: "label"
                model: editor.statuses
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Label { text: "Score (0–10)"; color: "#c0c0cc"; Layout.preferredWidth: 90 }
            SpinBox {
                id: scoreSpin
                objectName: "listScoreSpin"
                Layout.fillWidth: true
                from: 0; to: 10
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Label { text: "Progress"; color: "#c0c0cc"; Layout.preferredWidth: 90 }
            SpinBox {
                id: progressSpin
                objectName: "listProgressSpin"
                Layout.fillWidth: true
                from: 0
                to: editor.episodeCount > 0 ? editor.episodeCount : 9999
                editable: true
            }
        }
    }
}
