// Mirrors the JSON shapes returned by the Go daemon's /api/* and
// /api/runyx/* routes (see internal/api/*.go). Field casing is kept
// exactly as the server sends it: conversations/messages are
// unmarshaled straight from Go struct fields with no json tags
// (PascalCase), while registry/dashboard/permissions use explicit
// snake_case json tags. This is the same split web/src/lib/api.ts
// documents and lives with (no codegen ties the two together — kept in
// sync by hand).

export interface Conversation {
  ID: string;
  Title: string;
  Model: string;
  Folder: string;
  Pinned: boolean;
  ProjectID: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface Message {
  ID: string;
  ConversationID: string;
  ParentID: string;
  BranchID: string;
  Role: string;
  Content: string;
  CreatedAt: string;
}

export interface DashboardModel {
  name: string;
  port: number;
  idle_seconds: number;
}

export interface DashboardStats {
  loaded_models: DashboardModel[];
  total_ram_mb: number;
  total_vram_mb: number;
  backend: string;
}

export interface ModelEntry {
  name: string;
  modified_at: string;
  size: number;
  digest: string;
}

export interface DeviceInfo {
  id: string;
  name: string;
  created_at: string;
}

export interface PermissionsSnapshot {
  shell_agent: {
    enabled: boolean;
    work_dir: string;
    allow_prefixes: string[];
    deny_prefixes: string[];
  };
  vision_enabled: boolean;
  voice_enabled: boolean;
  web_search_on: boolean;
  devices: DeviceInfo[];
}

export type HardwareFit = "comfortable" | "ok" | "tight" | "too_large";

export interface RegistryEntry {
  id: string;
  name: string;
  publisher: string;
  family: string;
  params_b: number;
  repo: string;
  file: string;
  size_bytes: number;
  license: string;
  mmproj_file?: string;
  hf_url: string;
  description: string;
  capabilities: string[];
  context_length: number;
  installed: boolean;
  fit: HardwareFit;
  pull_spec: string;
}

export interface ChatMessageIn {
  role: string;
  content: string;
}

// Mirrors the JSON body of POST /api/runyx/pairing/qrcode-matrix: the
// same QR code the web UI renders as a PNG `<img>`, as a raw module
// grid (true = dark module) for a client with no image decoder.
export interface PairingQR {
  url: string;
  modules: boolean[][];
}

// Mirrors internal/approval.Request (Go). A pending request awaiting a
// human decision from whichever client is attached (TUI, web, or a
// paired device). Hash binds the decision to this exact content.
export interface ApprovalRequest {
  id: string;
  kind: string;
  summary: string;
  detail: string;
  hash: string;
  created_at: string;
  expires_at: string;
}
