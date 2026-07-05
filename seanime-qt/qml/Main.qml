import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: window
    width: 1100
    height: 760
    visible: true
    title: "Seanime-Qt"
    color: "#0e0e12"

    // Auto-connect to the default local server on startup.
    Component.onCompleted: app.connectToServer(hostField.text, portField.text, tokenField.text)

    // Push the detail page whenever the controller opens an anime.
    Connections {
        target: app
        function onAnimeOpened() {
            stack.push(detailComponent)
        }
    }

    header: ToolBar {
        background: Rectangle { color: "#17171f" }
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 12
            anchors.rightMargin: 12
            spacing: 8

            Label {
                text: "Seanime-Qt"
                color: "#ffffff"
                font.pixelSize: 18
                font.bold: true
            }

            Item { Layout.preferredWidth: 16 }

            TextField {
                id: hostField
                text: "127.0.0.1"
                Layout.preferredWidth: 130
                placeholderText: "host"
            }
            TextField {
                id: portField
                text: "43211"
                Layout.preferredWidth: 70
                placeholderText: "port"
            }
            TextField {
                id: tokenField
                Layout.preferredWidth: 150
                placeholderText: "token (optional)"
                echoMode: TextInput.Password
            }
            Button {
                text: "Connect"
                onClicked: app.connectToServer(hostField.text, portField.text, tokenField.text)
            }

            Item { Layout.fillWidth: true }

            Rectangle {
                width: 10; height: 10; radius: 5
                color: app.connectionStatus === "connected" ? "#3ecf5b"
                     : app.connectionStatus === "connecting" ? "#e0b341"
                     : "#e05a5a"
            }
            Label {
                text: app.connectionStatus
                color: "#c8c8d0"
                font.pixelSize: 13
            }
        }
    }

    // Error banner
    Rectangle {
        id: banner
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: visible ? 34 : 0
        visible: app.errorMessage.length > 0
        color: "#5a1f1f"
        z: 10
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 12
            anchors.rightMargin: 8
            Label {
                Layout.fillWidth: true
                text: app.errorMessage
                color: "#ffd9d9"
                elide: Text.ElideRight
            }
            ToolButton {
                text: "✕"
                onClicked: app.refresh()  // retry the library fetch
            }
        }
    }

    StackView {
        id: stack
        anchors.fill: parent
        anchors.topMargin: banner.height
        initialItem: libraryComponent
    }

    Component {
        id: libraryComponent
        LibraryView {}
    }

    Component {
        id: detailComponent
        DetailView {
            onBack: stack.pop()
        }
    }
}
