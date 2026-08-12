package heimdall

import (
	"errors"
	"net/http"

	"github.com/frey788/heimdall/dashboard"
	"github.com/frey788/heimdall/installer/config"
	"github.com/frey788/heimdall/store/memory"
	"github.com/frey788/heimdall/watchers"
	"google.golang.org/grpc"
)

type InstallOptions struct {
	DashboardPath          string
	PINEnabled             bool
	PIN                    string
	SensitiveMetadataKeys  []string
	InMemoryMaxEventBuffer int
}

type Runtime struct {
	Config    config.DashboardConfig
	Store     *memory.Store
	Wiring    *watchers.Wiring
	Dashboard http.Handler
}

func Install(options InstallOptions) (*Runtime, error) {
	dashboardConfig, err := config.BuildEmbeddedConfig(config.InstallerInput{
		DashboardPath: options.DashboardPath,
		PINEnabled:    options.PINEnabled,
		PIN:           options.PIN,
	})
	if err != nil {
		return nil, err
	}

	store := memory.NewStore(options.InMemoryMaxEventBuffer)

	wiringOptions := []watchers.Option{watchers.WithEmitter(store)}
	if len(options.SensitiveMetadataKeys) > 0 {
		wiringOptions = append(wiringOptions, watchers.WithSensitiveMetadataKeys(options.SensitiveMetadataKeys...))
	}
	wiring := watchers.NewWiring(wiringOptions...)

	handler := dashboard.NewHandler(store, dashboard.HandlerOptions{
		Protection: dashboardConfig.Protection,
	})

	return &Runtime{
		Config:    dashboardConfig,
		Store:     store,
		Wiring:    wiring,
		Dashboard: handler,
	}, nil
}

func (r *Runtime) Mount(mux *http.ServeMux) error {
	if mux == nil {
		return errors.New("mux is required")
	}

	dashboardPath := r.Config.Path
	mux.Handle(dashboardPath+"/", http.StripPrefix(dashboardPath, r.Dashboard))
	mux.Handle(dashboardPath, http.RedirectHandler(dashboardPath+"/", http.StatusTemporaryRedirect))
	return nil
}

func (r *Runtime) HTTPMiddleware(next http.Handler) http.Handler {
	return r.Wiring.HTTPMiddleware(next)
}

func (r *Runtime) GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return r.Wiring.GRPCUnaryServerInterceptor()
}

func (r *Runtime) GRPCStreamServerInterceptor() grpc.StreamServerInterceptor {
	return r.Wiring.GRPCStreamServerInterceptor()
}

func (r *Runtime) GRPCUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return r.Wiring.GRPCUnaryClientInterceptor()
}

func (r *Runtime) GRPCStreamClientInterceptor() grpc.StreamClientInterceptor {
	return r.Wiring.GRPCStreamClientInterceptor()
}
