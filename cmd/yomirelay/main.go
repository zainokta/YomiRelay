package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"yomirelay/internal/api"
	"yomirelay/internal/dialogue"
	"yomirelay/internal/events"
	"yomirelay/internal/games"
	"yomirelay/internal/hook"
	"yomirelay/internal/platform"
	"yomirelay/internal/receiver"
	"yomirelay/internal/steam"
	"yomirelay/internal/translation"
	"yomirelay/internal/web"
)

type Config struct {
	HTTPAddr string
	UDPAddr  string
}

func ConfigFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	config := Config{HTTPAddr: getenv("YOMIRELAY_HTTP_ADDR"), UDPAddr: getenv("YOMIRELAY_UDP_ADDR")}
	if config.HTTPAddr == "" {
		config.HTTPAddr = "127.0.0.1:17321"
	}
	if config.UDPAddr == "" {
		config.UDPAddr = "127.0.0.1:17322"
	}
	if err := validateLoopbackAddress(config.HTTPAddr); err != nil {
		return Config{}, fmt.Errorf("HTTP address: %w", err)
	}
	if err := validateLoopbackAddress(config.UDPAddr); err != nil {
		return Config{}, fmt.Errorf("UDP address: %w", err)
	}
	return config, nil
}

func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid host:port address: %w", err)
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	return fmt.Errorf("address host %q is not loopback", host)
}

func RootHandler(apiHandler, staticHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})
}

func Run(ctx context.Context, config Config, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}
	locator := platform.NewSteamLocator()
	roots, err := locator.FindSteamRoots()
	if err != nil {
		return fmt.Errorf("find Steam roots: %w", err)
	}
	logger.Printf("Steam roots: %v", roots)
	discover := func() ([]steam.Installation, error) { return steam.Discover(roots) }
	manager := hook.Manager{}
	store := dialogue.NewStore(1000, time.Now)
	broker := events.NewBroker(64)
	registry := games.NewRegistry(discover, manager.Installed, store.Activity)
	if err := registry.Refresh(); err != nil {
		return fmt.Errorf("discover games: %w", err)
	}
	logger.Printf("discovered %d Ren'Py games", len(registry.List()))
	codex := translation.New("codex")
	apiHandler := api.New(api.Dependencies{Games: registry, Hooks: manager, Store: store, Broker: broker, Translator: codex.Translate, Logger: logger})
	listener, err := receiver.Listen(ctx, config.UDPAddr, func(value dialogue.Dialogue) {
		store.Append(value)
		broker.Publish(value)
	})
	if err != nil {
		return err
	}
	server := &http.Server{Addr: config.HTTPAddr, Handler: RootHandler(apiHandler, web.Handler())}
	serverErr := make(chan error, 1)
	listenerErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	go func() { listenerErr <- listener.Wait() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		shutdownErr := server.Shutdown(shutdownCtx)
		cancel()
		_ = listener.Close()
		if shutdownErr != nil {
			return shutdownErr
		}
		return nil
	case err := <-serverErr:
		_ = listener.Close()
		<-listenerErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-listenerErr:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = server.Shutdown(shutdownCtx)
		cancel()
		return err
	}
}

func main() {
	logger := log.New(os.Stderr, "[backend] ", log.LstdFlags)
	config, err := ConfigFromEnv(os.Getenv)
	if err != nil {
		logger.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := Run(ctx, config, logger); err != nil {
		logger.Fatal(err)
	}
}
