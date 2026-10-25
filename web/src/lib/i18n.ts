// Single source of UI strings. English-only for now (see
// docs/DECISIONS.md: "UI and docs in English only") — this mechanism
// (a `Lang` union + one dict per locale) is kept exactly so a future
// locale is just a new union member and a new `dict` entry, not a
// rewrite. Voice *recognition* (item 25) is unrelated to this file and
// keeps understanding whatever language the user speaks.
export type Lang = "en";

const dict: Record<Lang, Record<string, string>> = {
  en: {
    newChat: "New conversation",
    conversations: "Conversations",
    send: "Send",
    placeholder: "Type a message...",
    pin: "Pin",
    unpin: "Unpin",
    export: "Export",
    search: "Search...",
    dashboard: "Dashboard",
    model: "Model",
    tokensPerSec: "tokens/s",
    loadedModels: "Loaded models",
    ram: "RAM",
    vram: "VRAM",
    backend: "Backend",
    commandPalette: "Command palette",
    theme: "Theme",
    light: "Light",
    dark: "Dark",
    system: "System",
    attachFile: "Attach a file",
    noConversations: "No conversations yet. Start typing!",
    editAndFork: "Edit (forks a branch)",
    branches: "Branches",
    reduceMotion: "Reduce motion",
    recordVoice: "Talk (push-to-talk)",
    recording: "Recording... click to stop",
    listenReply: "Listen to reply",
    stopListening: "Stop playback",
    navChat: "Chat",
    navModels: "Models",
    navPairing: "Pairing",
    navSettings: "Settings",
    modelsInstalled: "Installed models",
    pullModel: "Pull a model",
    pullModelPlaceholder: "e.g. Qwen/Qwen2.5-0.5B-Instruct-GGUF",
    pullButton: "Pull",
    size: "Size",
    modified: "Modified",
    pairingTitle: "Pair a phone",
    pairingDesc: "Scan this QR code with the Runyx Android app to pair this device.",
    pairingGenerate: "Generate QR code",
    settingsTitle: "Settings & permissions",
    settingsAppearance: "Appearance",
    settingsShellAgent: "Shell agent",
    settingsEnabled: "Enabled",
    settingsDisabled: "Disabled",
    settingsVision: "Vision",
    settingsVoice: "Voice",
    settingsWebSearch: "Web search",
    settingsDevices: "Paired devices",
    settingsRevoke: "Revoke",
    settingsNoDevices: "No paired devices.",
  },
};

export function detectLang(): Lang {
  return "en";
}

export function t(lang: Lang, key: string): string {
  return dict[lang][key] ?? key;
}
