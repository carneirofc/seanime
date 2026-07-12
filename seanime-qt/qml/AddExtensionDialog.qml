import QtQuick
import QtQuick.Controls
import QtQuick.Dialogs
import QtQuick.Layouts

// Adds an extension from a manifest URL or a local manifest file, mirroring the
// web "Add extension" flow: paste a manifest URL (or browse for a local .json),
// "Find" previews the extension it describes, then "Install" installs it.
// Closes itself once the install succeeds.
Dialog {
    id: dialog
    objectName: "addExtensionDialog"
    modal: true
    title: "Add an extension"
    anchors.centerIn: Overlay.overlay
    width: 480
    closePolicy: Popup.CloseOnEscape

    // The previewed extension (or {} when nothing has been fetched yet).
    readonly property var preview: app.extensionPreview
    readonly property bool hasPreview: preview && (preview.name !== undefined && preview.name !== "")

    // Start clean each time it opens; clear the shared preview state on close.
    onOpened: {
        app.clearExtensionPreview()
        manifestField.text = ""
        manifestField.forceActiveFocus()
    }
    onClosed: app.clearExtensionPreview()

    // The controller signals a successful install: dismiss the dialog.
    Connections {
        target: app
        function onExtensionInstalled() { dialog.close() }
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

    contentItem: ColumnLayout {
        spacing: 12

        Label {
            Layout.fillWidth: true
            text: "Paste a link to an extension manifest (.json), or browse for a local file. Find it first to review what it is, then install."
            color: Theme.textDim
            font.pixelSize: Theme.fontSm
            wrapMode: Text.WordWrap
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            AppTextField {
                id: manifestField
                objectName: "extensionManifestField"
                Layout.fillWidth: true
                placeholderText: "https://example.com/extension.json or a local file"
                onAccepted: if (text.trim().length > 0) app.fetchExtensionPreview(text)
            }
            AppButton {
                objectName: "extensionBrowseButton"
                text: "Browse"
                iconName: "external-link"
                onClicked: manifestFileDialog.open()
            }
            AppButton {
                objectName: "extensionFindButton"
                text: "Find"
                iconName: "search"
                enabled: !app.extensionFetching && manifestField.text.trim().length > 0
                onClicked: app.fetchExtensionPreview(manifestField.text)
            }
        }

        // Local manifest file picker. FileDialog returns a file:// URL, which the
        // server accepts directly as a manifest URI. Selecting a file immediately
        // previews it (as if the user typed the path and clicked Find).
        FileDialog {
            id: manifestFileDialog
            objectName: "extensionManifestFileDialog"
            title: "Select an extension manifest"
            nameFilters: ["JSON files (*.json)", "All files (*)"]
            onAccepted: {
                var uri = selectedFile.toString()
                manifestField.text = uri
                app.fetchExtensionPreview(uri)
            }
        }

        Label {
            Layout.fillWidth: true
            visible: app.extensionFetching
            text: "Fetching extension…"
            color: Theme.textMuted
            font.pixelSize: Theme.fontSm
        }

        // ---- preview card ----
        Rectangle {
            Layout.fillWidth: true
            visible: dialog.hasPreview
            radius: Theme.radius
            color: Theme.inset
            border.color: Theme.border
            implicitHeight: previewCol.implicitHeight + 20

            ColumnLayout {
                id: previewCol
                anchors.fill: parent
                anchors.margins: 10
                spacing: 6

                RowLayout {
                    Layout.fillWidth: true
                    spacing: 8
                    Label {
                        text: dialog.hasPreview ? dialog.preview.name : ""
                        color: Theme.textStrong
                        font.pixelSize: Theme.fontLg
                        font.bold: true
                        elide: Text.ElideRight
                    }
                    Label {
                        visible: dialog.hasPreview && dialog.preview.version !== undefined
                        text: dialog.hasPreview ? ("v" + dialog.preview.version) : ""
                        color: Theme.textMuted
                        font.pixelSize: Theme.fontSm
                    }
                    Item { Layout.fillWidth: true }
                    Label {
                        visible: dialog.hasPreview && dialog.preview.author !== undefined && dialog.preview.author !== ""
                        text: dialog.hasPreview ? ("by " + dialog.preview.author) : ""
                        color: Theme.textMuted
                        font.pixelSize: Theme.fontSm
                    }
                }
                Label {
                    Layout.fillWidth: true
                    visible: dialog.hasPreview && dialog.preview.description !== undefined && dialog.preview.description !== ""
                    text: dialog.hasPreview ? dialog.preview.description : ""
                    color: Theme.textDim
                    font.pixelSize: Theme.fontSm
                    wrapMode: Text.WordWrap
                }
            }
        }

        // ---- actions ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            Item { Layout.fillWidth: true }
            AppButton {
                objectName: "extensionAddCancelButton"
                text: "Cancel"
                onClicked: dialog.close()
            }
            AppButton {
                objectName: "extensionAddInstallButton"
                visible: dialog.hasPreview
                text: app.extensionInstalling ? "Installing…" : "Install"
                iconName: "download"
                enabled: !app.extensionInstalling && manifestField.text.trim().length > 0
                onClicked: app.installExtension(manifestField.text)
            }
        }
    }
}
