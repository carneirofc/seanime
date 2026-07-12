import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Extensions / providers page, mirroring the web frontend: an "Installed" tab
// listing the installed extensions (with enable/disable and uninstall), and a
// "Marketplace" tab to browse and install from the default repository. The
// "Add extension" button opens a dialog to install from an arbitrary manifest URL.
Item {
    id: root

    // "installed" | "marketplace"
    property string currentTab: "installed"

    readonly property var typeOptions: [
        { label: "All types", value: "" },
        { label: "Torrent providers", value: "anime-torrent-provider" },
        { label: "Manga providers", value: "manga-provider" },
        { label: "Streaming providers", value: "onlinestream-provider" },
        { label: "Plugins", value: "plugin" },
        { label: "Custom sources", value: "custom-source" }
    ]

    // Load both lists when the page opens (installed drives the marketplace's
    // "Installed" markers, so fetch it too even though it starts hidden).
    Component.onCompleted: {
        app.loadExtensions()
        app.loadMarketplace()
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        // ---- header: title + tabs + add ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 10

            Label {
                text: "Extensions"
                color: Theme.textStrong
                font.pixelSize: Theme.fontXxl
                font.bold: true
            }

            Item { width: 8 }

            // Segmented tab switch.
            AppButton {
                objectName: "extInstalledTab"
                text: "Installed"
                iconName: "puzzle"
                checkable: true
                checked: root.currentTab === "installed"
                onClicked: root.currentTab = "installed"
            }
            AppButton {
                objectName: "extMarketplaceTab"
                text: "Marketplace"
                iconName: "building-store"
                checkable: true
                checked: root.currentTab === "marketplace"
                onClicked: root.currentTab = "marketplace"
            }

            Item { Layout.fillWidth: true }

            AppButton {
                objectName: "extAddButton"
                text: "Add extension"
                iconName: "plus"
                onClicked: addDialog.open()
            }
        }

        // =====================================================================
        // Installed tab
        // =====================================================================
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: root.currentTab === "installed"
            spacing: 12

            RowLayout {
                Layout.fillWidth: true
                spacing: 8
                Label {
                    Layout.fillWidth: true
                    text: "Providers and plugins installed on your server."
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontSm
                }
                AppButton {
                    objectName: "extReloadSourceButton"
                    text: "Reload from source"
                    iconName: "download"
                    onClicked: app.reloadExtensionsFromSource()
                }
                AppButton {
                    objectName: "extReloadButton"
                    text: "Reload"
                    iconName: "refresh"
                    onClicked: app.reloadExtensions()
                }
                AppButton {
                    objectName: "extRefreshButton"
                    text: "Refresh"
                    iconName: "refresh"
                    onClicked: app.loadExtensions()
                }
            }

            Label {
                Layout.fillWidth: true
                visible: app.extensionsLoading
                text: "Loading extensions…"
                color: Theme.textMuted
                font.pixelSize: Theme.fontLg
                horizontalAlignment: Text.AlignHCenter
            }
            Label {
                Layout.fillWidth: true
                visible: !app.extensionsLoading && installedList.count === 0
                text: "No extensions installed. Add one from the Marketplace or via a manifest URL."
                color: Theme.textMuted
                font.pixelSize: Theme.fontLg
                horizontalAlignment: Text.AlignHCenter
            }

            ListView {
                id: installedList
                objectName: "installedExtensionList"
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                spacing: 8
                model: app.installedExtensionModel
                ScrollBar.vertical: ScrollBar {}
                delegate: ExtensionDelegate {
                    width: installedList.width
                    marketplace: false
                }
            }
        }

        // =====================================================================
        // Marketplace tab
        // =====================================================================
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: root.currentTab === "marketplace"
            spacing: 12

            RowLayout {
                Layout.fillWidth: true
                spacing: 8
                AppTextField {
                    id: marketSearch
                    objectName: "marketplaceSearchField"
                    Layout.fillWidth: true
                    placeholderText: "Search the marketplace…"
                    onTextEdited: app.setMarketplaceSearch(text)
                }
                AppComboBox {
                    id: marketTypeCombo
                    objectName: "marketplaceTypeCombo"
                    width: 180
                    textRole: "label"
                    valueRole: "value"
                    model: root.typeOptions
                    onActivated: app.setMarketplaceType(currentValue)
                }
                AppButton {
                    objectName: "marketplaceRefreshButton"
                    text: "Refresh"
                    iconName: "refresh"
                    onClicked: app.loadMarketplace()
                }
            }

            Label {
                Layout.fillWidth: true
                visible: app.marketplaceLoading
                text: "Loading marketplace…"
                color: Theme.textMuted
                font.pixelSize: Theme.fontLg
                horizontalAlignment: Text.AlignHCenter
            }
            Label {
                Layout.fillWidth: true
                visible: !app.marketplaceLoading && marketList.count === 0
                text: "No extensions match. Try a different search or type filter."
                color: Theme.textMuted
                font.pixelSize: Theme.fontLg
                horizontalAlignment: Text.AlignHCenter
            }

            ListView {
                id: marketList
                objectName: "marketplaceExtensionList"
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                spacing: 8
                model: app.marketplaceExtensionModel
                ScrollBar.vertical: ScrollBar {}
                delegate: ExtensionDelegate {
                    width: marketList.width
                    marketplace: true
                }
            }
        }
    }

    AddExtensionDialog {
        id: addDialog
    }
}
