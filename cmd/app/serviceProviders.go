package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	sql2 "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/go-jose/go-jose/v4"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	internalAudit "github.com/Thatooine/loyalty-points-app/internal/pkg/audit"
	internalAuth "github.com/Thatooine/loyalty-points-app/internal/pkg/authentication"
	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	internalWallet "github.com/Thatooine/loyalty-points-app/internal/pkg/wallets"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audit"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallets"
)

// ServiceProviders holds the wired-up service implementations used by the
// server adaptors and middleware.
type ServiceProviders struct {
	DB                         *sql.DB
	TransactionManager         sql2.TxManager
	UserRepository             pkgUsers.UserRepository
	AccountRepository          pkgAccounts.AccountRepository
	TransactionRepository      pkgWallet.TransactionRepository
	AuditEntryRepository       pkgAudit.AuditEntryRepository
	WalletService              pkgWallet.WalletService
	UserRegistrationService    pkgUsers.UserRegistrationService
	EmailPasswordAuthenticator pkgAuth.EmailPasswordAuthenticator
	AccessTokenIssuer          pkgAuth.AccessTokenIssuer
	AccessTokenValidator       pkgAuth.AccessTokenValidator
}

// Close releases resources held by the service providers.
func (s *ServiceProviders) Close() error {
	return s.DB.Close()
}

// NewServiceProviders constructs all service implementations from the given
// configuration.
func NewServiceProviders(ctx context.Context, config *Config, secureConfig *SecureConfig) (*ServiceProviders, error) {
	// open the Postgres database and apply any pending schema migrations
	db, err := postgres.NewClient(ctx, config.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("could not create postgres client: %w", err)
	}
	if err := postgres.Migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("could not migrate database: %w", err)
	}

	// repositories and the transaction manager, all behind their ports
	transactionManager := postgres.NewPostgresTxManager(db)
	userRepository := internalUsers.NewUserRepositoryImpl(db)
	accountRepository := internalAccounts.NewAccountRepositoryImpl(db)
	transactionRepository := internalWallet.NewTransactionRepositoryImpl(db)
	auditEntryRepository := internalAudit.NewAuditEntryRepositoryImpl(db)

	// parse the RSA private key used to sign and verify access tokens
	jwtPrivateKey, err := parseRSAPrivateKey(secureConfig.JWTPrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("could not parse JWT private key: %w", err)
	}

	// create a signer for issuing access tokens
	tokenSigner, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jwtPrivateKey},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create token signer: %w", err)
	}

	// services that compose the repositories (mirrors the house wiring order)
	accessTokenService := internalAuth.NewAccessTokenServiceImpl(tokenSigner, &jwtPrivateKey.PublicKey)
	emailPasswordAuthenticator := internalAuth.NewEmailPasswordAuthenticatorImpl(
		userRepository,
		accessTokenService,
	)

	walletService := internalWallet.NewWalletServiceImpl(
		transactionManager,
		accountRepository,
		transactionRepository,
		auditEntryRepository,
	)

	userRegistrationService := internalUsers.NewUserRegistrationServiceImpl(
		transactionManager,
		userRepository,
		accountRepository,
		accessTokenService,
	)

	return &ServiceProviders{
		DB:                         db,
		TransactionManager:         transactionManager,
		UserRepository:             userRepository,
		AccountRepository:          accountRepository,
		TransactionRepository:      transactionRepository,
		AuditEntryRepository:       auditEntryRepository,
		WalletService:              walletService,
		UserRegistrationService:    userRegistrationService,
		EmailPasswordAuthenticator: emailPasswordAuthenticator,
		AccessTokenIssuer:          accessTokenService,
		AccessTokenValidator:       accessTokenService,
	}, nil
}

// parseRSAPrivateKey decodes a PKCS#8 PEM-encoded RSA private key.
func parseRSAPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("could not decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse PKCS#8 private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA key")
	}

	return rsaKey, nil
}
