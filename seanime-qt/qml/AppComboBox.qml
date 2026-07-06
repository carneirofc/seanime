import QtQuick
import QtQuick.Controls

// A ComboBox with a pointing-hand cursor on hover. Drop-in for ComboBox.
ComboBox {
    HoverHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        cursorShape: Qt.PointingHandCursor
    }
}
