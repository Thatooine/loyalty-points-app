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
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	internalAudit "github.com/Thatooine/loyalty-points-app/internal/pkg/audit"
	internalAuth "github.com/Thatooine/loyalty-points-app/internal/pkg/authentication"
	internalRateLimiting "github.com/Thatooine/loyalty-points-app/internal/pkg/rateLimiting"
	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	internalWallet "github.com/Thatooine/loyalty-points-app/internal/pkg/wallets"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	pkgRateLimiting "github.com/Thatooine/loyalty-points-app/pkg/rateLimiting"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallets"
)

type Dependencies struct {
	DB                         *sql.DB
	RedisClient                *redis.Client
	RateLimiter                pkgRateLimiting.RedisTokenBucketRateLimiter
	TransactionManager         sql2.TxManager
	UserRepository             pkgUsers.UserRepository
	AccountRepository          pkgAccounts.AccountRepository
	TransactionRepository      pkgWallet.TransactionRepository
	AuditEntryRepository       pkgAudit.AuditEntryRepository
	AuditService               pkgAudit.AuditService
	WalletService              pkgWallet.WalletService
	AccountService             pkgAccounts.AccountService
	AccountOpener              pkgAccounts.AccountOpener
	UserRegistrationService    pkgUsers.UserRegistrationService
	EmailPasswordAuthenticator pkgAuth.EmailPasswordAuthenticator
	AccessTokenIssuer          pkgAuth.AccessTokenIssuer
	AccessTokenValidator       pkgAuth.AccessTokenValidator
	LogoutService              pkgAuth.LogoutService
}

func (s *Dependencies) Close() error {
	if s.RedisClient != nil {
		if err := s.RedisClient.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close redis client")
		}
	}
	return s.DB.Close()
}

func NewDependencies(ctx context.Context, config *Config, secureConfig *SecureConfig) (*Dependencies, error) {
	db, err := postgres.NewClient(ctx, config.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("could not create postgres client: %w", err)
	}
	if err := postgres.Migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("could not migrate database: %w", err)
	}

	// Rate limiter (Redis-backed). Outside local the config layer already
	// requires REDIS_URI, so a connection failure here is fatal. In local an
	// unreachable / unset Redis degrades gracefully: rate limiting is disabled
	// (RateLimiter stays nil) so the zero-config dev flow and the integration
	// suite still run without standing up Redis.
	var redisClient *redis.Client
	var rateLimiter pkgRateLimiting.RedisTokenBucketRateLimiter
	if config.RedisURI != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: config.RedisURI})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			if config.Environment != EnvLocal {
				return nil, fmt.Errorf("could not connect to redis: %w", err)
			}
			log.Warn().Err(err).Str("redis", config.RedisURI).Msg("redis unreachable in local; rate limiting disabled")
			_ = redisClient.Close()
			redisClient = nil
		} else {
			rateLimiter = internalRateLimiting.NewRedisRateLimiterImpl(redisClient)
		}
	}

	transactionManager := postgres.NewPostgresTxManager(db)
	userRepository := internalUsers.NewUserRepositoryImpl(db)
	accountRepository := internalAccounts.NewAccountRepositoryImpl(db)
	transactionRepository := internalWallet.NewTransactionRepositoryImpl(db)
	auditEntryRepository := internalAudit.NewAuditEntryRepositoryImpl(db)

	jwtPrivateKey, err := parseRSAPrivateKey(secureConfig.JWTPrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("could not parse JWT private key: %w", err)
	}

	tokenSigner, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jwtPrivateKey},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create token signer: %w", err)
	}

	accessTokenService := internalAuth.NewAccessTokenServiceImpl(tokenSigner, &jwtPrivateKey.PublicKey, userRepository)
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

	accountService := internalAccounts.NewAccountServiceImpl(accountRepository)

	auditService := internalAudit.NewAuditServiceImpl(auditEntryRepository)

	accountOpener := internalAccounts.NewAccountOpenerServiceImpl(accountRepository)

	logoutService := internalAuth.NewLogoutServiceImpl(userRepository)

	userRegistrationService := internalUsers.NewUserRegistrationServiceImpl(
		transactionManager,
		userRepository,
		accountRepository,
		accessTokenService,
	)

	return &Dependencies{
		DB:                         db,
		RedisClient:                redisClient,
		RateLimiter:                rateLimiter,
		TransactionManager:         transactionManager,
		UserRepository:             userRepository,
		AccountRepository:          accountRepository,
		TransactionRepository:      transactionRepository,
		AuditEntryRepository:       auditEntryRepository,
		AuditService:               auditService,
		WalletService:              walletService,
		AccountService:             accountService,
		AccountOpener:              accountOpener,
		UserRegistrationService:    userRegistrationService,
		EmailPasswordAuthenticator: emailPasswordAuthenticator,
		AccessTokenIssuer:          accessTokenService,
		AccessTokenValidator:       accessTokenService,
		LogoutService:              logoutService,
	}, nil
}

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
