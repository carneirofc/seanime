import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Left navigation, mirroring the web frontend: logo on top, a vertical
// icon+label menu, and a user/login chip pinned to the bottom.
Rectangle {
    id: sidebar
    color: "#141420"

    // Current top-level page id, driven by Main; used to highlight the active item.
    property string currentPage: "home"

    signal navigate(string page)
    signal loginRequested()

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // ---- logo ----
        Item {
            Layout.fillWidth: true
            Layout.preferredHeight: 64
            Label {
                anchors.centerIn: parent
                text: "Seanime"
                color: "#ffffff"
                font.pixelSize: 20
                font.bold: true
            }
        }

        // ---- nav items ----
        Repeater {
            model: [
                { pageId: "home",     label: "Home",     glyph: "🏠" },
                { pageId: "discover", label: "Discover", glyph: "🧭" },
                { pageId: "search",   label: "Search",   glyph: "🔍" },
                { pageId: "profile",  label: "Profile",  glyph: "👤" },
            ]
            delegate: Rectangle {
                id: navItem
                required property var modelData
                objectName: "nav_" + modelData.pageId
                Layout.fillWidth: true
                Layout.preferredHeight: 44
                Layout.leftMargin: 8
                Layout.rightMargin: 8
                radius: 8
                readonly property bool active: sidebar.currentPage === modelData.pageId
                color: active ? "#2a2a3a" : (navHover.hovered ? "#20202c" : "transparent")

                function activate() { sidebar.navigate(modelData.pageId) }

                // Keyboard: reachable via Tab, activated with Enter/Return/Space.
                activeFocusOnTab: true
                Keys.onReturnPressed: navItem.activate()
                Keys.onEnterPressed: navItem.activate()
                Keys.onSpacePressed: navItem.activate()

                // Accessibility: announce as a button with its label; expose the
                // press action so assistive tech can trigger navigation.
                Accessible.role: Accessible.Button
                Accessible.name: modelData.label
                Accessible.description: "Go to the " + modelData.label + " page"
                Accessible.focusable: true
                Accessible.onPressAction: navItem.activate()

                // Visible focus ring for keyboard users.
                border.width: navItem.activeFocus ? 2 : 0
                border.color: "#3ea6ff"

                Row {
                    anchors.left: parent.left
                    anchors.leftMargin: 12
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 12
                    Label { text: modelData.glyph; font.pixelSize: 17 }
                    Label {
                        text: modelData.label
                        color: active ? "#ffffff" : "#c8c8d0"
                        font.pixelSize: 14
                        font.bold: active
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                // Active accent bar on the left edge.
                Rectangle {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: 3; height: parent.height * 0.5; radius: 2
                    color: "#3ea6ff"
                    visible: parent.active
                }

                HoverHandler { id: navHover; cursorShape: Qt.PointingHandCursor }
                TapHandler { onTapped: navItem.activate() }
            }
        }

        Item { Layout.fillHeight: true }  // push the rest to the bottom

        // ---- connection status ----
        RowLayout {
            Layout.fillWidth: true
            Layout.leftMargin: 20
            Layout.rightMargin: 12
            Layout.bottomMargin: 8
            spacing: 8
            Rectangle {
                width: 9; height: 9; radius: 5
                color: app.connectionStatus === "connected" ? "#3ecf5b"
                     : app.connectionStatus === "connecting" ? "#e0b341"
                     : "#e05a5a"
            }
            Label {
                text: app.connectionStatus
                color: "#8a8a96"
                font.pixelSize: 12
            }
        }

        // ---- user / login chip ----
        Rectangle {
            id: userChip
            objectName: "userChip"
            Layout.fillWidth: true
            Layout.preferredHeight: 56
            Layout.margins: 8
            radius: 8
            readonly property bool loggedIn: app.username.length > 0
            readonly property bool hasAvatar: loggedIn && app.avatarUrl.length > 0
            color: userHover.hovered ? "#20202c" : "#1a1a22"

            function activate() {
                if (loggedIn) sidebar.navigate("profile")
                else sidebar.loginRequested()
            }

            // Keyboard reachable + activatable.
            activeFocusOnTab: true
            Keys.onReturnPressed: userChip.activate()
            Keys.onEnterPressed: userChip.activate()
            Keys.onSpacePressed: userChip.activate()

            // Accessibility: name reflects the current state/action.
            Accessible.role: Accessible.Button
            Accessible.name: loggedIn ? app.username : "Log in with AniList"
            Accessible.description: loggedIn
                ? "Open your profile"
                : "Sign in to AniList"
            Accessible.focusable: true
            Accessible.onPressAction: userChip.activate()

            border.width: userChip.activeFocus ? 2 : 0
            border.color: "#3ea6ff"

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 10
                anchors.rightMargin: 10
                spacing: 10

                // Avatar (or a login glyph when signed out).
                Rectangle {
                    width: 34; height: 34; radius: 17
                    color: "#0e0e12"
                    clip: true
                    Image {
                        anchors.fill: parent
                        source: app.avatarUrl
                        visible: userChip.hasAvatar
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                    }
                    Label {
                        anchors.centerIn: parent
                        visible: !userChip.hasAvatar
                        text: userChip.loggedIn ? app.username.charAt(0).toUpperCase() : "🔑"
                        color: "#8a8a96"
                        font.pixelSize: userChip.loggedIn ? 16 : 14
                    }
                }

                Label {
                    Layout.fillWidth: true
                    text: userChip.loggedIn ? app.username : "Log in with AniList"
                    color: userChip.loggedIn ? "#e6e6ee" : "#9ad0ff"
                    font.pixelSize: 13
                    elide: Text.ElideRight
                }
            }

            HoverHandler { id: userHover; cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: userChip.activate() }
        }
    }
}
