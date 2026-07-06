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
            GradientStop { position: 0.0; color: "#000e0e12" }
            GradientStop { position: 1.0; color: "#e60e0e12" }
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
            color: "#1a1a22"
            border.color: "#0e0e12"; border.width: 4
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
                color: "#8a8a96"
                font.pixelSize: 42
            }
        }

        Label {
            objectName: "profileNameLabel"
            Layout.alignment: Qt.AlignHCenter
            text: root.loggedIn ? app.username : "Not logged in"
            color: "#ffffff"
            font.pixelSize: 24
            font.bold: true
        }

        Label {
            Layout.alignment: Qt.AlignHCenter
            text: root.loggedIn
                  ? "Signed in to AniList"
                  : "Use “Log in with AniList” in the header to sign in."
            color: "#9ad0ff"
            font.pixelSize: 13
        }

        // Simple stat card.
        Rectangle {
            Layout.alignment: Qt.AlignHCenter
            Layout.topMargin: 8
            implicitWidth: 200; implicitHeight: 72
            radius: 8
            color: "#1a1a22"
            ColumnLayout {
                anchors.centerIn: parent
                spacing: 2
                Label {
                    Layout.alignment: Qt.AlignHCenter
                    text: app.libraryCount
                    color: "#ffffff"
                    font.pixelSize: 26
                    font.bold: true
                }
                Label {
                    Layout.alignment: Qt.AlignHCenter
                    text: "titles in library"
                    color: "#8a8a96"
                    font.pixelSize: 12
                }
            }
        }
    }
}
