package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Thatooine/loyalty-points-app/pkg/logger"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	gorillaJSON "github.com/gorilla/rpc/v2/json2"
	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

func setupRPCServer(providers ServiceProviders) {
	port := 8080

	router := mux.NewRouter()

	// All JSON-RPC services — public and protected — are exposed on the single
	// /api endpoint. The authorization middleware gates each request by the
	// method it calls: public methods (e.g. login) pass through untouched,
	// while every other method must present a valid token and a role permitted
	// to call it. Business adaptors — wallet, accounts, audit — are added to
	// this list as they are built.
	services := []jsonrpc.Service{
		authentication.NewEmailPasswordAuthenticatorJSONRPCAdaptor(providers.EmailPasswordAuthenticator),
		users.NewUserRegistrationServiceJSONRPCAdaptor(providers.UserRegistrationService),
	}

	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(logger.Middleware)
	apiRouter.Use(authorization.NewAuthorizationMiddleware(providers.AccessTokenValidator, authorization.DefaultPermissions()))
	apiRouter.Handle("", newJSONRPCServer(services))

	// start the http server
	log.Info().Msg(fmt.Sprintf("Starting JSON-RPC server on: 0.0.0.0:%d", port))
	go func() {
		server := &http.Server{
			Handler:      router,
			Addr:         fmt.Sprintf("0.0.0.0:%d", port),
			WriteTimeout: 150 * time.Second,
			ReadTimeout:  150 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("error starting json rpc server")
		}
	}()
}

// newJSONRPCServer creates a JSON-RPC server with the JSON codec registered and
// the given services mounted under their Name().
func newJSONRPCServer(services []jsonrpc.Service) *rpc.Server {
	jsonRPCServer := rpc.NewServer()
	jsonRPCServer.RegisterCodec(gorillaJSON.NewCodec(), "application/json")

	for _, service := range services {
		log.Info().Msg("\tRegistering: " + service.Name())
		if err := jsonRPCServer.RegisterService(service, service.Name()); err != nil {
			log.Fatal().Err(err).Msg("could not register: " + service.Name())
		}
	}

	return jsonRPCServer
}
