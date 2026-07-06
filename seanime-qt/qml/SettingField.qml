import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs

// One settings field, rendered by ``type``:
//   switch            — label + toggle on a row
//   text / password   — a single-line (optionally masked) input
//   int               — a bounded SpinBox
//   combo             — an enum dropdown (closed, or ``editable`` to allow custom
//                       values); options are [{label, value}]
//   file / dir        — a path input with a Browse… button opening a native
//                       file / folder dialog
//
// Exposes a single ``value`` that SettingsView seeds from the current settings and
// reads back on save. User edits write through ``value`` via user-only signals
// (onToggled / onTextEdited / onValueModified / onActivated) so seeding it from a
// binding never loops, and an unrecognised combo value is preserved (never
// overwritten on load).
//
// Optional ``validation`` ("port" | "host" | "url") drives an inline error and the
// read-only ``valid`` flag, which SettingsView checks before saving.
ColumnLayout {
    id: field

    property string label: ""
    property string description: ""
    property string type: "text"      // switch|text|password|int|combo|file|dir
    property var value                 // current value (bool / string / int)
    property int from: 0
    property int to: 999999
    property string placeholder: ""
    property var options: []           // combo: [{label, value}]
    property bool editable: false      // combo: allow free-text custom values
    property string validation: ""     // "" | "port" | "host" | "url"

    // ---- validation ----
    function validationError(v) {
        if (field.validation === "") return ""
        var s = ("" + (v === undefined || v === null ? "" : v)).trim()
        if (s === "") return ""  // empty is allowed (fields are optional)
        if (field.validation === "port") {
            if (!/^\d+$/.test(s) || Number(s) < 1 || Number(s) > 65535)
                return "Enter a port between 1 and 65535."
        } else if (field.validation === "host") {
            if (!/^[A-Za-z0-9._\-:\[\]]+$/.test(s))
                return "Enter a valid host or IP address."
        } else if (field.validation === "url") {
            if (!/^https?:\/\/.+/.test(s))
                return "Enter a URL starting with http:// or https://."
        }
        return ""
    }
    readonly property string errorText: validationError(field.value)
    readonly property bool valid: errorText === ""

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
        AppSwitch {
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

    AppTextField {
        id: tf
        visible: field.type === "text" || field.type === "password"
        Layout.fillWidth: true
        Layout.maximumWidth: 440
        text: field.value === undefined || field.value === null ? "" : "" + field.value
        placeholderText: field.placeholder
        color: field.valid ? Theme.text : Theme.danger
        echoMode: field.type === "password" ? TextInput.Password : TextInput.Normal
        onTextEdited: field.value = text
    }

    AppSpinBox {
        id: sb
        visible: field.type === "int"
        editable: true
        from: field.from
        to: field.to
        // Only ints reach here; other types share ``value`` as string/bool, so
        // coerce anything non-numeric to 0 to avoid a binding type error.
        value: (typeof field.value === "number") ? field.value : 0
        onValueModified: field.value = value
    }

    // ---- combo: closed enum, or editable for custom values ----
    AppComboBox {
        id: cb
        visible: field.type === "combo"
        Layout.fillWidth: true
        Layout.maximumWidth: 440
        model: field.options
        textRole: "label"
        valueRole: "value"
        editable: field.editable
        // Display-only seed: bind the selection to the current value without
        // writing back, so an unlisted value is preserved until the user picks.
        currentIndex: cb.indexOfValue(field.value)
        editText: field.editable
                  ? (field.value === undefined || field.value === null ? "" : "" + field.value)
                  : ""
        onActivated: field.value = cb.currentValue
        onAccepted: if (field.editable) field.value = editText
        onEditTextChanged: if (field.editable && cb.acceptableInput) field.value = editText
    }

    // ---- file / directory picker: path input + Browse… ----
    RowLayout {
        visible: field.type === "file" || field.type === "dir"
        Layout.fillWidth: true
        Layout.maximumWidth: 560
        spacing: Theme.spacingSm

        AppTextField {
            id: pathField
            Layout.fillWidth: true
            text: field.value === undefined || field.value === null ? "" : "" + field.value
            placeholderText: field.placeholder
            color: field.valid ? Theme.text : Theme.danger
            onTextEdited: field.value = text
        }
        AppButton {
            objectName: "browse_" + field.label
            text: "Browse…"
            onClicked: if (dialogLoader.item) dialogLoader.item.open()
        }
    }

    // The picker dialogs are only created for file/dir fields.
    Loader {
        id: dialogLoader
        active: field.type === "file" || field.type === "dir"
        sourceComponent: field.type === "dir" ? folderDialogComp : fileDialogComp
    }
    Component {
        id: fileDialogComp
        FileDialog {
            title: "Select " + field.label
            onAccepted: field.value = app.urlToLocalPath("" + selectedFile)
        }
    }
    Component {
        id: folderDialogComp
        FolderDialog {
            title: "Select " + field.label
            onAccepted: field.value = app.urlToLocalPath("" + selectedFolder)
        }
    }

    // ---- inline validation error ----
    Label {
        visible: field.type !== "switch" && field.errorText.length > 0
        text: field.errorText
        color: Theme.danger
        font.pixelSize: Theme.fontSm
        wrapMode: Text.WordWrap
        Layout.fillWidth: true
    }
}
