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
    color: Theme.bg

    // Switch the main content to a top-level page (like the web sidebar does).
    // Replace immediately: an animated replace leaves the outgoing page
    // un-destroyed (its own populate/hover animations keep it "busy", so the
    // transition never finalizes and the old view lingers, visible, behind the
    // new one — leaking a view per navigation). Immediate tears it down at once.
    function showPage(comp, id) {
        sidebar.currentPage = id
        stack.replace(null, comp, StackView.Immediate)
    }

    // Push the client-local appearance prefs into the Theme singleton. Theme is a
    // singleton and can't read the `app` context property itself, so the root
    // window bridges it — here on startup and again whenever a pref changes.
    function applyUiPrefs() {
        Theme.uiScale = app.uiScale
        Theme.density = app.uiDensity
        Theme.mode = app.uiThemeMode
        Theme.accentBase = app.uiAccent
        Theme.posterScale = app.uiPosterScale
    }

    // Apply the persisted appearance prefs, then auto-connect to the local server.
    Component.onCompleted: {
        window.applyUiPrefs()
        app.connectToServer(hostField.text, portField.text, tokenField.text)
    }

    Connections {
        target: app
        // Re-apply appearance prefs to the Theme when they change (live restyle).
        function onUiPrefsChanged() { window.applyUiPrefs() }
        // Push the detail page on top of whatever page is showing.
        function onAnimeOpened() { stack.push(detailComponent) }
        // Manga: push the manga detail page, then the reader on top of it.
        function onMangaOpened() { stack.push(mangaDetailComponent) }
        function onChapterOpened() { stack.push(readerComponent) }
        // On successful login, dismiss the login page (back to the current root).
        function onLoginFinished() { if (stack.depth > 1) stack.pop(null) }
        // A genre chip was tapped: switch to the search page, which consumes the
        // pending genre in its Component.onCompleted and runs the query.
        function onGenreSearchRequested(genre) { window.showPage(searchComponent, "search") }
        function onTagSearchRequested(tag) { window.showPage(searchComponent, "search") }
    }

    // Slim top bar: server connection only (navigation lives in the sidebar).
    header: ToolBar {
        background: Rectangle { color: Theme.surfaceAlt }
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: Theme.spacing
            anchors.rightMargin: Theme.spacing
            spacing: Theme.spacingSm

            Label {
                text: "Server"
                color: Theme.textMuted
                font.pixelSize: Theme.fontMd
            }
            AppTextField {
                id: hostField
                objectName: "hostField"
                text: app.serverHost
                Layout.preferredWidth: 130
                placeholderText: "host"
                Accessible.name: "Server host"
            }
            AppTextField {
                id: portField
                objectName: "portField"
                text: app.serverPort
                Layout.preferredWidth: 70
                placeholderText: "port"
                Accessible.name: "Server port"
            }
            AppTextField {
                id: tokenField
                objectName: "tokenField"
                text: app.serverToken
                Layout.preferredWidth: 150
                placeholderText: "token (optional)"
                echoMode: TextInput.Password
                Accessible.name: "Server token, optional"
            }
            AppButton {
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
                else if (page === "manga") showPage(mangaLibraryComponent, "manga")
                else if (page === "discover") showPage(discoverComponent, "discover")
                else if (page === "search") showPage(searchComponent, "search")
                else if (page === "profile") showPage(profileComponent, "profile")
                else if (page === "settings") showPage(settingsComponent, "settings")
            }
            onLoginRequested: stack.push(loginComponent)
        }

        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0

            // Error banner — slides/fades in when an error is present.
            Rectangle {
                id: banner
                objectName: "errorBanner"
                readonly property bool showing: app.errorMessage.length > 0
                Layout.fillWidth: true
                Layout.preferredHeight: showing ? 34 : 0
                clip: true
                opacity: showing ? 1 : 0
                color: Theme.dangerFill

                Behavior on Layout.preferredHeight {
                    NumberAnimation { duration: Theme.durBase; easing.type: Theme.easeStandard }
                }
                Behavior on opacity { NumberAnimation { duration: Theme.durBase } }

                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: Theme.spacing
                    anchors.rightMargin: Theme.spacingSm
                    Label {
                        objectName: "errorLabel"
                        Layout.fillWidth: true
                        text: app.errorMessage
                        color: Theme.dangerText
                        elide: Text.ElideRight
                    }
                    AppToolButton {
                        iconName: "x"
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

                // Push (e.g. opening a detail page): new page slides in from the
                // right and fades up; the old one fades back.
                pushEnter: Transition {
                    ParallelAnimation {
                        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durSlow; easing.type: Theme.easeStandard }
                        NumberAnimation { property: "x"; from: 36; to: 0; duration: Theme.durSlow; easing.type: Theme.easeStandard }
                    }
                }
                pushExit: Transition {
                    NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Theme.durBase }
                }
                // Pop (Back): reverse of push.
                popEnter: Transition {
                    NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durBase }
                }
                popExit: Transition {
                    ParallelAnimation {
                        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Theme.durBase }
                        NumberAnimation { property: "x"; from: 0; to: 36; duration: Theme.durBase; easing.type: Theme.easeStandard }
                    }
                }
                // Replace (top-level page switches from the sidebar): fade the new
                // page in, but remove the old one instantly. An animated replaceExit
                // combined with replace(null, …) and the pages' own populate/hover
                // animations leaves the outgoing view un-destroyed (it lingers,
                // visible, behind the new page). Keeping the exit instant guarantees
                // the old page is torn down on every navigation.
                replaceEnter: Transition {
                    NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
                }
                replaceExit: Transition {}
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
        id: mangaLibraryComponent
        MangaLibraryView {
            onLoginRequested: stack.push(loginComponent)
        }
    }

    Component {
        id: mangaDetailComponent
        MangaDetailView {
            onBack: stack.pop()
        }
    }

    Component {
        id: readerComponent
        ReaderView {
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

    Component {
        id: settingsComponent
        SettingsView {}
    }
}
