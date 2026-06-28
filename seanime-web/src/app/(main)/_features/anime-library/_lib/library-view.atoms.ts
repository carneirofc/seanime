import { atom } from "jotai"
import { atomWithImmer } from "jotai-immer"

export const __library_viewAtom = atom<"base" | "detailed">("base")
export const __library_viewStateAtom = atomWithImmer<{ tagsPanelVisible: boolean, isAdult: boolean }>({ tagsPanelVisible: false, isAdult: false })