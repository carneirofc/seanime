import QtQuick
import QtQuick.Controls

// A standard Button that shows a pointing-hand cursor on hover, like the web
// frontend. Drop-in replacement for Button — inherits its full API. Using a
// wrapper keeps the cursor behaviour in one place (see also AppToolButton,
// AppComboBox) instead of repeating a HoverHandler on every button.
Button {
    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }
}
