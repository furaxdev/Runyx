package daemon

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/furaxdev/runyx/internal/api"
)

// NewHTTPServer builds the daemon's HTTP server: the Ollama/OpenAI API
// routes behind the device-token auth middleware, bound to
// d.Config.ListenAddr.
func (d *Daemon) NewHTTPServer() *http.Server {
	mux := http.NewServeMux()
	apiServer := api.NewServer(d.Store, d.Manager, d.HW)
	apiServer.History = d.History
	apiServer.Register(mux)
	extras := &api.RunyxExtras{
		History:     d.History,
		Transcriber: d.Transcriber,
		Vision:      d.Vision,
	}
	apiServer.RegisterRunyx(mux, extras)
	apiServer.RegisterKnowledge(mux, extras)

	agentExtras := &api.AgentExtras{Agent: d.Agent, Interpreter: d.Interpreter, Approvals: d.Approvals}
	agentExtras.RegisterAgent(mux)

	perms := &api.PermissionsCenter{
		Agent:         d.Agent,
		VisionEnabled: d.Vision != nil,
		VoiceEnabled:  d.Transcriber != nil,
		DeviceLister: func() []api.DeviceInfo {
			var out []api.DeviceInfo
			for _, dev := range d.Auth.List() {
				out = append(out, api.DeviceInfo{ID: dev.ID, Name: dev.Name, CreatedAt: dev.CreatedAt.Format(time.RFC3339)})
			}
			return out
		},
		DeviceRevoker: d.Auth.Revoke,
	}
	perms.RegisterPermissions(mux)

	listenHost, portStr, _ := net.SplitHostPort(d.Config.ListenAddr)
	port, _ := strconv.Atoi(portStr)
	voiceExtras := &api.VoiceExtras{
		Transcriber: d.Transcriber,
		Synthesizer: d.Synthesizer,
		PairPort:    port,
		ListenHost:  listenHost,
		IssueToken: func(deviceName string) (string, error) {
			dev, err := d.Auth.Issue(deviceName)
			return dev.Token, err
		},
	}
	voiceExtras.RegisterVoice(mux)

	remoteExtras := &api.RemoteExtras{Router: d.Remote}
	remoteExtras.RegisterRemote(mux)

	m6 := &api.M6Extras{Plugins: d.Plugins, Scheduler: d.Scheduler, Server: apiServer}
	m6.RegisterM6(mux)

	regExtras := &api.RegistryExtras{Server: apiServer}
	regExtras.RegisterRegistry(mux)

	registerStaticUI(mux)

	return &http.Server{
		Addr:              d.Config.ListenAddr,
		Handler:           d.Auth.Middleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

// Serve starts the HTTP server and the idle-eviction loop, blocking until
// ctx is cancelled, at which point it shuts everything down gracefully.
func (d *Daemon) Serve(ctx context.Context) error {
	d.StartEvictionLoop(ctx)
	defer d.Shutdown()

	srv := d.NewHTTPServer()
	ln, err := net.Listen("tcp", d.Config.ListenAddr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
