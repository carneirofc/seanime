import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Profile: the logged-in AniList viewer (avatar, name, banner) plus a local
// library count. Data comes from /api/v1/status (see AppController._apply_user).
Item {
    id: root

    readonly property bool loggedIn: app.username.length > 0

    // Banner backdrop.
    Image {
        id: banner
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 200
        source: app.bannerUrl
        visible: app.bannerUrl.length > 0
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        clip: true
    }
    Rectangle {
        anchors.fill: banner
        visible: banner.visible
        gradient: Gradient {
            GradientStop { position: 0.0; color: Qt.rgba(Theme.bg.r, Theme.bg.g, Theme.bg.b, 0.0) }
            GradientStop { position: 1.0; color: Qt.rgba(Theme.bg.r, Theme.bg.g, Theme.bg.b, 0.9) }
        }
    }

    ColumnLayout {
        anchors.top: parent.top
        anchors.topMargin: 130
        anchors.horizontalCenter: parent.horizontalCenter
        width: Math.min(parent.width - 48, 640)
        spacing: 14

        // Avatar
        Rectangle {
            Layout.alignment: Qt.AlignHCenter
            width: 120; height: 120; radius: 60
            color: Theme.surface
            border.color: Theme.bg; border.width: 4
            clip: true
            Image {
                anchors.fill: parent
                source: app.avatarUrl
                visible: app.avatarUrl.length > 0
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
            }
            Label {
                anchors.centerIn: parent
                visible: app.avatarUrl.length === 0
                text: root.loggedIn ? app.username.charAt(0).toUpperCase() : "?"
                color: Theme.textMuted
                font.pixelSize: 42
            }
        }

        Label {
            objectName: "profileNameLabel"
            Layout.alignment: Qt.AlignHCenter
            text: root.loggedIn ? app.username : "Not logged in"
            color: Theme.textStrong
            font.pixelSize: Theme.fontHero
            font.bold: true
        }

        Label {
            Layout.alignment: Qt.AlignHCenter
            text: root.loggedIn
                  ? "Signed in to AniList"
                  : "Use “Log in with AniList” in the header to sign in."
            color: Theme.accentSoft
            font.pixelSize: Theme.fontMd
        }

        // Simple stat card.
        Rectangle {
            Layout.alignment: Qt.AlignHCenter
            Layout.topMargin: 8
            implicitWidth: 200; implicitHeight: 72
            radius: Theme.radius
            color: Theme.surface
            ColumnLayout {
                anchors.centerIn: parent
                spacing: 2
                Label {
                    Layout.alignment: Qt.AlignHCenter
                    text: app.libraryCount
                    color: Theme.textStrong
                    font.pixelSize: 26
                    font.bold: true
                }
                Label {
                    Layout.alignment: Qt.AlignHCenter
                    text: "titles in library"
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontSm
                }
            }
        }
    }
}
