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
	"github.com/Thatooine/loyalty-points-app/pkg/users"
	"github.com/Thatooine/loyalty-points-app/pkg/wallets"
)

func setupRPCServer(providers ServiceProviders) *http.Server {
	port := 8080

	router := mux.NewRouter()

	services := []jsonrpc.Service{
		authentication.NewEmailPasswordAuthenticatorJSONRPCAdaptor(providers.EmailPasswordAuthenticator),
		authentication.NewLogoutServiceJSONRPCAdaptor(providers.LogoutService),
		users.NewUserRegistrationServiceJSONRPCAdaptor(providers.UserRegistrationService),
		wallets.NewWalletServiceJSONRPCAdaptor(providers.WalletService),
		accounts.NewAccountServiceJSONRPCAdaptor(providers.AccountService),
		accounts.NewAccountOpenerJSONRPCAdaptor(providers.AccountOpener),
		audits.NewAuditServiceJSONRPCAdaptor(providers.AuditService),
	}

	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(logger.Middleware)
	apiRouter.Use(authorization.NewAuthorizationMiddleware(providers.AccessTokenValidator, authorization.DefaultPolicy()))
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
