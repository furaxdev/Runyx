package api

import (
	"net/http"

	"github.com/furaxdev/runyx/internal/agent"
)

// PermissionsCenter answers "what can each model/plugin/device do"
// (item 24): a read model of the capabilities currently wired up, plus
// device token management (already implemented by daemon.TokenStore —
// exposed here so the UI has one place to show and revoke everything).
type PermissionsCenter struct {
	Agent         *agent.Agent // nil if the shell agent isn't configured
	VisionEnabled bool
	VoiceEnabled  bool
	WebSearchOn   bool // whether web search is enabled by default for new conversations
	DeviceLister  func() []DeviceInfo
	DeviceRevoker func(id string) error
}

// DeviceInfo is the UI-safe view of a daemon.Device: never includes the
// actual bearer token.
type DeviceInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// PermissionsSnapshot is what GET /api/runyx/permissions returns.
type PermissionsSnapshot struct {
	ShellAgent struct {
		Enabled       bool     `json:"enabled"`
		WorkDir       string   `json:"work_dir"`
		AllowPrefixes []string `json:"allow_prefixes"`
		DenyPrefixes  []string `json:"deny_prefixes"`
	} `json:"shell_agent"`
	VisionEnabled bool         `json:"vision_enabled"`
	VoiceEnabled  bool         `json:"voice_enabled"`
	WebSearchOn   bool         `json:"web_search_on"`
	Devices       []DeviceInfo `json:"devices"`
}

// RegisterPermissions wires the permissions center onto mux.
func (pc *PermissionsCenter) RegisterPermissions(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/runyx/permissions", pc.handleGet)
	mux.HandleFunc("DELETE /api/runyx/permissions/devices/{id}", pc.handleRevokeDevice)
}

func (pc *PermissionsCenter) handleGet(w http.ResponseWriter, r *http.Request) {
	var snap PermissionsSnapshot
	if pc.Agent != nil {
		snap.ShellAgent.Enabled = true
		snap.ShellAgent.WorkDir = pc.Agent.Policy.WorkDir
		snap.ShellAgent.AllowPrefixes = pc.Agent.Policy.AllowPrefixes
		snap.ShellAgent.DenyPrefixes = pc.Agent.Policy.DenyPrefixes
	}
	snap.VisionEnabled = pc.VisionEnabled
	snap.VoiceEnabled = pc.VoiceEnabled
	snap.WebSearchOn = pc.WebSearchOn
	if pc.DeviceLister != nil {
		snap.Devices = pc.DeviceLister()
	}
	writeJSON(w, http.StatusOK, snap)
}

func (pc *PermissionsCenter) handleRevokeDevice(w http.ResponseWriter, r *http.Request) {
	if pc.DeviceRevoker == nil {
		writeError(w, http.StatusServiceUnavailable, errNotConfigured)
		return
	}
	if err := pc.DeviceRevoker(r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

var errNotConfigured = jsonErr("device management is not configured")

type jsonErr string

func (e jsonErr) Error() string { return string(e) }
