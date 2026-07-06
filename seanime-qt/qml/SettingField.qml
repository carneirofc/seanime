import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One settings field, rendered by ``type``: a switch (label + toggle on a row),
// or a stacked label + text/password/number control. Exposes a single ``value``
// that SettingsView seeds from the current settings and reads back on save.
//
// User edits write through ``value`` via the user-only signals (onToggled /
// onTextEdited / onValueModified) so seeding it from a binding never loops.
ColumnLayout {
    id: field

    property string label: ""
    property string description: ""
    property string type: "text"      // "switch" | "text" | "password" | "int"
    property var value                 // current value (bool / string / int)
    property int from: 0
    property int to: 999999
    property string placeholder: ""

    Layout.fillWidth: true
    spacing: Theme.spacingXs

    // ---- switch: label/description on the left, toggle on the right ----
    RowLayout {
        Layout.fillWidth: true
        visible: field.type === "switch"
        spacing: Theme.spacing

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2
            Label {
                text: field.label
                color: Theme.text
                font.pixelSize: Theme.fontBase
                Layout.fillWidth: true
                wrapMode: Text.WordWrap
            }
            Label {
                visible: field.description.length > 0
                text: field.description
                color: Theme.textMuted
                font.pixelSize: Theme.fontSm
                wrapMode: Text.WordWrap
                Layout.fillWidth: true
            }
        }
        Switch {
            id: sw
            objectName: "switch_" + field.label
            checked: !!field.value
            onToggled: field.value = checked
        }
    }

    // ---- non-switch: stacked label + description + control ----
    Label {
        visible: field.type !== "switch"
        text: field.label
        color: Theme.text
        font.pixelSize: Theme.fontBase
    }
    Label {
        visible: field.type !== "switch" && field.description.length > 0
        text: field.description
        color: Theme.textMuted
        font.pixelSize: Theme.fontSm
        wrapMode: Text.WordWrap
        Layout.fillWidth: true
    }
    TextField {
        id: tf
        visible: field.type === "text" || field.type === "password"
        Layout.fillWidth: true
        Layout.maximumWidth: 440
        text: field.value === undefined || field.value === null ? "" : "" + field.value
        placeholderText: field.placeholder
        color: Theme.text
        echoMode: field.type === "password" ? TextInput.Password : TextInput.Normal
        onTextEdited: field.value = text
    }
    SpinBox {
        id: sb
        visible: field.type === "int"
        editable: true
        from: field.from
        to: field.to
        value: field.value === undefined || field.value === null ? 0 : field.value
        onValueModified: field.value = value
    }
}
