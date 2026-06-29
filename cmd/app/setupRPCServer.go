package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Thatooine/loyalty-points-app/pkg/logger"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	gorillaJSON "github.com/gorilla/rpc/v2/json2"
	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/audits"
	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
	"github.com/Thatooine/loyalty-points-app/pkg/rateLimiting"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
	"github.com/Thatooine/loyalty-points-app/pkg/wallets"
)

// Rate-limit budgets. Token buckets: capacity is the burst, refillRate is the
// time to regain one token. Defaults are deliberately generous (a real
// deployment should tighten them via ops); the point here is a DoS / brute-force
// ceiling, not tight quota enforcement.
const (
	// IP limiter on the public auth methods — credential brute-force guard.
	authRateCapacity = 200
	authRateRefill   = 100 * time.Millisecond // ~10 sustained req/s/IP after burst

	// User limiter on authenticated traffic, keyed by UserID.
	userRateCapacity = 100
	userRateRefill   = 20 * time.Millisecond // ~50 sustained req/s/user after burst
)

// publicAuthMethods are the unauthenticated JSON-RPC methods the IP limiter
// throttles. They mirror the public set in authorization.DefaultPolicy().
var publicAuthMethods = map[string]bool{
	"EmailPasswordAuthenticator.Login": true,
	"UserRegistrationService.Register": true,
}

func setupRPCServer(deps Dependencies) *http.Server {
	port := 8080

	router := mux.NewRouter()

	services := []jsonrpc.Service{
		authentication.NewEmailPasswordAuthenticatorJSONRPCAdaptor(deps.EmailPasswordAuthenticator),
		authentication.NewLogoutServiceJSONRPCAdaptor(deps.LogoutService),
		users.NewUserRegistrationServiceJSONRPCAdaptor(deps.UserRegistrationService),
		wallets.NewWalletServiceJSONRPCAdaptor(deps.WalletService),
		accounts.NewAccountServiceJSONRPCAdaptor(deps.AccountService),
		accounts.NewAccountOpenerJSONRPCAdaptor(deps.AccountOpener),
		audits.NewAuditServiceJSONRPCAdaptor(deps.AuditService),
	}

	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(logger.Middleware)

	// IP limiter runs before auth so a flood of login/register attempts is shed
	// cheaply, before any JWT work. Skipped entirely when no rate limiter is
	// wired (local without Redis).
	if deps.RateLimiter != nil {
		apiRouter.Use(rateLimiting.NewIPRateLimiterMiddleware(
			deps.RateLimiter, publicAuthMethods, authRateCapacity, authRateRefill))
	}

	apiRouter.Use(authorization.NewAuthorizationMiddleware(deps.AccessTokenValidator, authorization.DefaultPolicy()))

	// User limiter runs after auth: it keys on the login claim the authorization
	// middleware puts on the context.
	if deps.RateLimiter != nil {
		apiRouter.Use(rateLimiting.NewUserRateLimiterMiddleware(
			deps.RateLimiter, userRateCapacity, userRateRefill))
	}

	apiRouter.Handle("", newJSONRPCServer(services))

	// start the http server
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Info().Msg(fmt.Sprintf("Starting JSON-RPC server on: %s", addr))

	server := &http.Server{
		Handler:           router,
		Addr:              addr,
		WriteTimeout:      150 * time.Second,
		ReadTimeout:       150 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Bind the listener up front so "ready to serve" is logged only once the
	// port is actually accepting connections (a bind failure logs and exits).
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("error starting json rpc server")
	}
	log.Info().Msg(fmt.Sprintf("JSON-RPC server ready to serve on: %s", addr))

	go func() {
		// ErrServerClosed is the expected signal from a graceful Shutdown, not a
		// failure, so it must not trigger Fatal.
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("error serving json rpc server")
		}
	}()

	return server
}

func newJSONRPCServer(services []jsonrpc.Service) *rpc.Server {
	jsonRPCServer := rpc.NewServer()
	// Without the error mapper gorilla collapses every non-json2 error to -32000;
	// jsonrpc.MapError preserves stable codes and the machine-readable data.reason.
	codec := gorillaJSON.NewCustomCodecWithErrorMapper(rpc.DefaultEncoderSelector, jsonrpc.MapError)
	jsonRPCServer.RegisterCodec(codec, "application/json")

	for _, service := range services {
		log.Info().Msg("\tRegistering: " + service.Name())
		if err := jsonRPCServer.RegisterService(service, service.Name()); err != nil {
			log.Fatal().Err(err).Msg("could not register: " + service.Name())
		}
	}

	return jsonRPCServer
}
