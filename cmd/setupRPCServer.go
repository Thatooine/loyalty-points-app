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
	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
)

func setupRPCServer(providers ServiceProviders) {
	port := 8080

	router := mux.NewRouter()

	// create a new json rpc server and register the JSON codec
	jsonRPCServer := rpc.NewServer()
	jsonRPCServer.RegisterCodec(gorillaJSON.NewCodec(), "application/json")

	// json rpc services exposed on the /api path
	services := []jsonrpc.Service{
		authentication.NewEmailPasswordAuthenticatorJSONRPCAdaptor(providers.EmailPasswordAuthenticator),
	}

	// register each service with the json rpc server
	for _, service := range services {
		log.Info().Msg("\tRegistering: " + service.Name())
		if err := jsonRPCServer.RegisterService(service, service.Name()); err != nil {
			log.Fatal().Err(err).Msg("could not register: " + service.Name())
		}
	}

	// create a sub-router at /api and mount the json rpc server on it
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(logger.Middleware)
	apiRouter.Handle("", jsonRPCServer)

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
