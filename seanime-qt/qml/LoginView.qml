import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtWebEngine

// Embedded AniList login. Loads the OAuth authorize URL in an in-app browser;
// when AniList redirects to the registered callback the access token arrives in
// the URL fragment, which the controller extracts (see AppController.handleCallback).
Item {
    id: root
    signal close()

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 44
            color: Theme.surfaceAlt
            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 12
                anchors.rightMargin: 8
                spacing: 8
                Label {
                    text: "Log in to AniList"
                    color: Theme.textStrong
                    font.pixelSize: Theme.fontLg
                    font.bold: true
                }
                Item { Layout.fillWidth: true }
                AppButton {
                    objectName: "loginCancelButton"
                    text: "Cancel"
                    onClicked: root.close()
                }
            }
        }

        WebEngineView {
            id: webView
            objectName: "loginWebView"
            Layout.fillWidth: true
            Layout.fillHeight: true
            url: app.anilistAuthorizeUrl()

            // Swallow the AniList page's own console spam (media/WebRTC/etc.).
            onJavaScriptConsoleMessage: function(level, message, lineNumber, sourceID) {}

            // Every navigation (including the final redirect to the callback) is
            // offered to the controller; once it consumes the token/error we're done.
            onUrlChanged: {
                if (app.handleCallback(webView.url.toString()))
                    root.close()
            }
        }
    }
}
