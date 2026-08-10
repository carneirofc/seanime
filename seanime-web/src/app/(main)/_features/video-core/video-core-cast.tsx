import { atom } from "jotai"

// Casting was an Electron-only feature; the desktop app has been removed.
// These stubs keep the VideoCore integration points inert on web.

export const vc_isCasting = atom(false)

export function useCastSubtitleRelay() {}

export function VideoCoreCastButton() {
    return null
}

export function CastPlaybackControls(_props: { onStop?: () => void }) {
    return null
}
