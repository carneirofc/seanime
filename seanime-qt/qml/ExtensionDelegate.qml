import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One extension row, used by both the Installed and Marketplace lists (the
// `marketplace` flag switches the action set). Shows the icon, name, metadata
// chips, author and description, plus context-appropriate actions:
//   - marketplace: Install (or an "Installed" chip)
//   - installed:   Enable/Disable, Uninstall (hidden for built-ins)
//   - invalid:     the failure reason + Uninstall
Rectangle {
    id: row

    // Roles from ExtensionModel.
    required property string extId
    required property string name
    required property string version
    required property string extType
    required property string typeLabel
    required property string language
    required property string lang
    required property string author
    required property string description
    required property string icon
    required property string website
    required property string readme
    required property string manifestUri
    required property bool isBuiltin
    required property bool disabled
    required property bool invalid
    required property string invalidReason
    required property bool installed

    // Set by the hosting view: true in the marketplace tab, false when installed.
    property bool marketplace: false

    height: content.implicitHeight + 20
    radius: Theme.radius
    color: hover.hovered ? Theme.surfaceHover : Theme.surface
    border.width: 1
    border.color: row.invalid ? Theme.danger
                : hover.hovered ? Theme.border
                : "transparent"
    Behavior on color { ColorAnimation { duration: Theme.durFast } }

    Accessible.role: Accessible.ListItem
    Accessible.name: row.name

    HoverHandler { id: hover }

    RowLayout {
        id: content
        anchors.fill: parent
        anchors.margins: 10
        spacing: 12

        // ---- icon (or a puzzle-piece fallback) ----
        Rectangle {
            Layout.alignment: Qt.AlignTop
            width: 40; height: 40
            radius: Theme.radiusSm
            color: Theme.inset
            clip: true
            Image {
                id: iconImage
                anchors.fill: parent
                source: row.icon
                visible: row.icon.length > 0 && status === Image.Ready
                fillMode: Image.PreserveAspectFit
                asynchronous: true
            }
            Icon {
                anchors.centerIn: parent
                visible: !iconImage.visible
                name: "puzzle"
                size: 20
                color: Theme.textMuted
            }
        }

        // ---- name, chips, description ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 6

            RowLayout {
                Layout.fillWidth: true
                spacing: 8
                Label {
                    text: row.name
                    color: Theme.text
                    font.pixelSize: Theme.fontBase
                    font.bold: true
                    elide: Text.ElideRight
                    Layout.maximumWidth: implicitWidth
                }
                Label {
                    visible: row.author.length > 0
                    text: "by " + row.author
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontSm
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
            }

            Flow {
                Layout.fillWidth: true
                spacing: 6
                Chip { visible: row.version.length > 0; text: "v" + row.version }
                Chip { visible: row.typeLabel.length > 0; text: row.typeLabel }
                Chip { visible: row.language.length > 0; text: row.language }
                Chip { visible: row.lang.length > 0 && row.lang !== "multi"; text: row.lang }
                Chip {
                    visible: row.isBuiltin
                    text: "Built-in"
                    icon: "lock"
                }
                Chip {
                    visible: !row.marketplace && row.disabled && !row.invalid
                    text: "Disabled"
                    textColor: Theme.textMuted
                    fillColor: Theme.elevated
                }
                Chip {
                    visible: row.invalid
                    text: "Invalid"
                    icon: "alert-triangle"
                    textColor: Theme.dangerText
                    fillColor: Theme.dangerFill
                }
                Chip {
                    visible: row.marketplace && row.installed
                    text: "Installed"
                    icon: "circle-check"
                    textColor: Theme.successText
                    fillColor: Theme.successFill
                }
            }

            Label {
                visible: row.description.length > 0
                Layout.fillWidth: true
                text: row.description
                color: Theme.textDim
                font.pixelSize: Theme.fontSm
                wrapMode: Text.WordWrap
                maximumLineCount: 2
                elide: Text.ElideRight
            }

            Label {
                visible: row.invalid && row.invalidReason.length > 0
                Layout.fillWidth: true
                text: row.invalidReason
                color: Theme.dangerText
                font.pixelSize: Theme.fontSm
                wrapMode: Text.WordWrap
                maximumLineCount: 2
                elide: Text.ElideRight
            }
        }

        // ---- actions ----
        RowLayout {
            Layout.alignment: Qt.AlignVCenter
            spacing: 6

            // External links (website / readme) — shown when present.
            AppToolButton {
                visible: row.website.length > 0
                iconName: "external-link"
                onClicked: Qt.openUrlExternally(row.website)
                Accessible.name: "Open website"
            }
            AppToolButton {
                visible: row.readme.length > 0
                iconName: "book"
                onClicked: Qt.openUrlExternally(row.readme)
                Accessible.name: "Open documentation"
            }

            // Marketplace: install (or nothing, if already installed).
            AppButton {
                visible: row.marketplace && !row.installed
                text: "Install"
                iconName: "download"
                enabled: !app.extensionInstalling
                onClicked: app.installExtension(row.manifestUri)
            }

            // Installed: enable/disable toggle (not for invalid entries).
            AppButton {
                visible: !row.marketplace && !row.invalid && !row.isBuiltin
                text: row.disabled ? "Enable" : "Disable"
                iconName: "power"
                onClicked: app.setExtensionDisabled(row.extId, !row.disabled)
            }

            // Installed: uninstall (external extensions only, never built-ins).
            AppButton {
                visible: !row.marketplace && !row.isBuiltin
                text: "Uninstall"
                iconName: "trash"
                onClicked: app.uninstallExtension(row.extId)
            }
        }
    }
}
