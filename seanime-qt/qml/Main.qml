import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: window
    objectName: "mainWindow"
    width: 1100
    height: 760
    visible: true
    title: "Seanime-Qt"
    color: "#0e0e12"

    // Switch the main content to a top-level page (like the web sidebar does).
    function showPage(comp, id) {
        sidebar.currentPage = id
        stack.replace(null, comp)
    }

    // Auto-connect to the default local server on startup.
    Component.onCompleted: app.connectToServer(hostField.text, portField.text, tokenField.text)

    Connections {
        target: app
        // Push the detail page on top of whatever page is showing.
        function onAnimeOpened() { stack.push(detailComponent) }
        // On successful login, dismiss the login page (back to the current root).
        function onLoginFinished() { if (stack.depth > 1) stack.pop(null) }
    }

    // Slim top bar: server connection only (navigation lives in the sidebar).
    header: ToolBar {
        background: Rectangle { color: "#17171f" }
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 12
            anchors.rightMargin: 12
            spacing: 8

            Label {
                text: "Server"
                color: "#8a8a96"
                font.pixelSize: 13
            }
            TextField {
                id: hostField
                objectName: "hostField"
                text: "127.0.0.1"
                Layout.preferredWidth: 130
                placeholderText: "host"
                Accessible.name: "Server host"
            }
            TextField {
                id: portField
                objectName: "portField"
                text: "43211"
                Layout.preferredWidth: 70
                placeholderText: "port"
                Accessible.name: "Server port"
            }
            TextField {
                id: tokenField
                objectName: "tokenField"
                Layout.preferredWidth: 150
                placeholderText: "token (optional)"
                echoMode: TextInput.Password
                Accessible.name: "Server token, optional"
            }
            Button {
                objectName: "connectButton"
                text: "Connect"
                onClicked: app.connectToServer(hostField.text, portField.text, tokenField.text)
            }
            Item { Layout.fillWidth: true }
        }
    }

    // Body: sidebar + content.
    RowLayout {
        anchors.fill: parent
        spacing: 0

        Sidebar {
            id: sidebar
            Layout.preferredWidth: 210
            Layout.fillHeight: true
            onNavigate: function(page) {
                if (page === "home") showPage(libraryComponent, "home")
                else if (page === "discover") showPage(discoverComponent, "discover")
                else if (page === "search") showPage(searchComponent, "search")
                else if (page === "profile") showPage(profileComponent, "profile")
            }
            onLoginRequested: stack.push(loginComponent)
        }

        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0

            // Error banner
            Rectangle {
                id: banner
                objectName: "errorBanner"
                Layout.fillWidth: true
                Layout.preferredHeight: visible ? 34 : 0
                visible: app.errorMessage.length > 0
                color: "#5a1f1f"
                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: 12
                    anchors.rightMargin: 8
                    Label {
                        objectName: "errorLabel"
                        Layout.fillWidth: true
                        text: app.errorMessage
                        color: "#ffd9d9"
                        elide: Text.ElideRight
                    }
                    ToolButton {
                        text: "✕"
                        onClicked: app.refresh()  // retry the library fetch
                        Accessible.name: "Dismiss error and retry"
                        Accessible.description: "Clears the error banner and refetches your library"
                    }
                }
            }

            StackView {
                id: stack
                objectName: "stack"
                Layout.fillWidth: true
                Layout.fillHeight: true
                // Clip so push/replace slide animations stay within the content
                // area and don't paint over the sidebar or error banner.
                clip: true
                initialItem: libraryComponent
            }
        }
    }

    Component {
        id: libraryComponent
        LibraryView {
            onLoginRequested: stack.push(loginComponent)
        }
    }

    Component {
        id: detailComponent
        DetailView {
            onBack: stack.pop()
        }
    }

    Component {
        id: loginComponent
        LoginView {
            onClose: stack.pop()
        }
    }

    Component {
        id: searchComponent
        SearchView {}
    }

    Component {
        id: discoverComponent
        DiscoverView {}
    }

    Component {
        id: profileComponent
        ProfileView {}
    }
}
