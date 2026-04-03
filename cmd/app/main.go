package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // registers pprof handlers on http.DefaultServeMux
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	appconfig "vibechat/internal/config"
	"vibechat/internal/delivery"
	deliveryhttp "vibechat/internal/delivery/http"
	"vibechat/internal/delivery/http/handler"
	mw "vibechat/internal/delivery/http/middleware"
	"vibechat/internal/delivery/ws"
	"vibechat/internal/infrastructure/cache"
	"vibechat/internal/infrastructure/storage"
	"vibechat/internal/infrastructure/storage/cached"
	chatUC "vibechat/internal/usecase/chat"
	messageUC "vibechat/internal/usecase/message"
	userUC "vibechat/internal/usecase/user"
	"vibechat/utils/hasher"
	"vibechat/utils/jwt"
	"vibechat/utils/logger"
)

// API is the lifecycle interface for the HTTP/WS server.
type API interface {
	Run() error
	Stop(ctx context.Context) error
}

type application struct {
	cfg   *appconfig.AppConfig
	store *storage.Storage
	cache cache.Cache
}

type server struct {
	httpSrv      *http.Server
	pprofSrv     *http.Server // non-nil only when pprof is enabled
	httpCfg      deliveryhttp.Config
	pprofEnabled bool
	pprofAddr    string
}

func (s *server) Run() error {
	if s.pprofEnabled {
		s.pprofSrv = &http.Server{Addr: s.pprofAddr}
		go func() {
			log.Info().Str("addr", s.pprofAddr).Msg("pprof server started")
			if err := s.pprofSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("pprof server stopped")
			}
		}()
	}

	if s.httpCfg.TLSEnabled() {
		log.Info().Str("addr", s.httpSrv.Addr).Msg("https server listening")
		if err := s.httpSrv.ListenAndServeTLS(s.httpCfg.TLS.CertFile, s.httpCfg.TLS.KeyFile); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	} else {
		log.Info().Str("addr", s.httpSrv.Addr).Msg("http server listening")
		if err := s.httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

func (s *server) Stop(ctx context.Context) error {
	if s.pprofSrv != nil {
		_ = s.pprofSrv.Shutdown(ctx)
	}
	return s.httpSrv.Shutdown(ctx)
}

func main() {
	cfgPath := flag.String("config", "configs/app.yaml", "path to config file")
	flag.Parse()

	cfg, err := appconfig.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a, api, err := newApplication(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("init failed")
	}
	defer a.close()

	go func() {
		if err := api.Run(); err != nil {
			log.Error().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := api.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}

	log.Info().Msg("shutdown complete")
}

func newApplication(ctx context.Context, cfg *appconfig.AppConfig) (*application, API, error) {
	a := &application{cfg: cfg}

	store, err := storage.New(context.Background(), cfg.Storage)
	if err != nil {
		return nil, nil, fmt.Errorf("storage: %w", err)
	}
	a.store = store
	if p := cfg.Storage.Postgres; p != nil {
		log.Info().Str("host", p.Host).Int("port", p.Port).Msg("postgres connected")
	}
	if m := cfg.Storage.MySQL; m != nil {
		log.Info().Str("host", m.Host).Int("port", m.Port).Msg("mysql connected")
	}

	var c cache.Cache
	if cfg.Cache != nil {
		c, err = cache.New(ctx, *cfg.Cache)
		if err != nil {
			return nil, nil, fmt.Errorf("cache: %w", err)
		}
		a.cache = c
		log.Info().Str("type", string(cfg.Cache.Type)).Msg("cache initialized")
	} else {
		log.Info().Msg("cache disabled")
	}

	jwtMgr := jwt.New(cfg.JWT)
	pwdHash := hasher.New(cfg.Hasher)

	var ttl cache.TTLConfig
	if cfg.Cache != nil {
		ttl = cfg.Cache.TTL
	}

	cachedChats := cached.NewChatRepository(store.Chats, c, ttl.Chat.Duration, ttl.Member.Duration)
	usersUC := userUC.New(cached.NewUserRepository(store.Users, c, ttl.User.Duration), pwdHash, jwtMgr)
	chatsUC := chatUC.New(cachedChats, store.Users)
	messagesUC := messageUC.New(store.Messages, cachedChats)

	mux := http.NewServeMux()

	if cfg.Delivery.IsEnabled(delivery.WS) {
		hub := ws.NewHub()
		wsHandler := ws.NewHandler(hub, chatsUC, usersUC, cfg.Delivery.HTTP.AllowedOrigins, cfg.Delivery.WS)
		mux.Handle("GET /ws", mw.Auth(jwtMgr)(http.HandlerFunc(wsHandler.ServeHTTP)))
		log.Info().Msg("ws api enabled")
	}

	rl := cfg.Delivery.HTTP.RateLimit
	if cfg.Delivery.IsEnabled(delivery.HTTP) {
		deliveryhttp.SetupRoutes(mux, deliveryhttp.Deps{
			User:        handler.NewUserHandler(usersUC),
			Chat:        handler.NewChatHandler(chatsUC),
			Message:     handler.NewMessageHandler(messagesUC),
			JWT:         jwtMgr,
			Health:      healthCheck(a),
			RateLimiter: mw.RateLimit(ctx, rl.MaxRequests, rl.Window.Duration),
		})
		log.Info().Msg("http api enabled")
	}

	var httpHandler http.Handler = mux
	httpHandler = mw.LimitBody(cfg.Delivery.HTTP.MaxBodyBytes)(httpHandler)
	httpHandler = mw.CORS(cfg.Delivery.HTTP.AllowedOrigins)(httpHandler)
	httpHandler = mw.RequestID()(httpHandler)
	httpHandler = mw.RequestLogger()(httpHandler)
	httpHandler = mw.Recovery()(httpHandler)

	addr := fmt.Sprintf("%s:%d", cfg.Delivery.HTTP.Host, cfg.Delivery.HTTP.Port)
	srv := &server{
		httpSrv: &http.Server{
			Addr:         addr,
			Handler:      httpHandler,
			ReadTimeout:  cfg.Delivery.HTTP.ReadTimeout.Duration,
			WriteTimeout: cfg.Delivery.HTTP.WriteTimeout.Duration,
			IdleTimeout:  cfg.Delivery.HTTP.IdleTimeout.Duration,
		},
		httpCfg:      cfg.Delivery.HTTP,
		pprofEnabled: cfg.Profile.Enabled,
		pprofAddr:    cfg.Profile.Addr,
	}

	return a, srv, nil
}

func (a *application) close() {
	if a.store != nil {
		a.store.Close()
	}
	if a.cache != nil {
		if err := a.cache.Close(); err != nil {
			log.Error().Err(err).Msg("cache close error")
		}
	}
}

func healthCheck(a *application) deliveryhttp.HealthFunc {
	return func(ctx context.Context) error {
		if a.store != nil {
			if err := a.store.Ping(ctx); err != nil {
				return fmt.Errorf("storage: %w", err)
			}
		}
		if a.cache != nil {
			if err := a.cache.Ping(ctx); err != nil {
				return fmt.Errorf("cache: %w", err)
			}
		}
		return nil
	}
}
