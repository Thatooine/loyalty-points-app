package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/go-jose/go-jose/v4"

	internalAuth "github.com/Thatooine/loyalty-points-app/internal/pkg/authentication"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

// ServiceProviders holds the wired-up service implementations used by the
// server adaptors and middleware.
type ServiceProviders struct {
	EmailAndPasswordAuthService pkgAuth.EmailAndPasswordAuthService
	AccessTokenService          pkgAuth.AccessTokenService
}

// NewServiceProviders constructs all service implementations from the given
// configuration.
func NewServiceProviders(ctx context.Context, config *Config, secureConfig *SecureConfig) (*ServiceProviders, error) {
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

	accessTokenService := internalAuth.NewAccessTokenServiceImpl(tokenSigner, &jwtPrivateKey.PublicKey)
	emailAndPasswordAuthService := internalAuth.NewEmailAndPasswordAuthServiceImpl(
		accessTokenService,
	)

	return &ServiceProviders{
		EmailAndPasswordAuthService: emailAndPasswordAuthService,
		AccessTokenService:          accessTokenService,
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
