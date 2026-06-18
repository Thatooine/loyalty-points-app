package main

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// EnvLocal is the default environment. It is the only environment in which the
// baked dev defaults (DSN + JWT signing key) are permitted; every other value
// is treated as a real deployment that must supply its own secrets via env vars
// or startup fails.
const EnvLocal = "local"

func init() {
	viper.MustBindEnv("Environment", "APP_ENV")
	viper.MustBindEnv("JWTPrivateKeyPEM", "JWT_PRIVATE_KEY_PEM")
	viper.MustBindEnv("PostgresDSN", "POSTGRES_DSN")
	viper.MustBindEnv("RedisURI", "REDIS_URI")
}

type Config struct {
	Environment string
	PostgresDSN string
	// RedisURI backs the rate limiter. Empty disables rate limiting in local;
	// outside local it is required (fail closed) so a deployment can't silently
	// run without per-IP / per-user throttling.
	RedisURI string
}

// SecureConfig holds secret configuration, kept separate from Config so secrets
// are not logged alongside ordinary settings.
type SecureConfig struct {
	JWTPrivateKeyPEM string
}

func GetConfig(configFileName string) (*Config, *SecureConfig) {
	viper.SetDefault("Environment", EnvLocal)

	if configFileName != "" {
		viper.SetConfigFile(configFileName)
		if err := viper.ReadInConfig(); err != nil {
			log.Warn().Err(err).Msg("could not read configuration file")
		}
	}

	// Baked defaults are scoped to the local environment only. Outside local we
	// deliberately leave secrets unset so the fail-closed check below catches a
	// missing key/DSN rather than silently signing tokens with a key from source
	// control.
	env := viper.GetString("Environment")
	if env == EnvLocal {
		viper.SetDefault("RedisURI", "localhost:6379")
		viper.SetDefault("PostgresDSN", "postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable")
		viper.SetDefault("JWTPrivateKeyPEM", `-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQCsTXWAE1NG6IVw
8Q/QkUbSGsNaHAgmtAXHvR79BTXziNMWU7iKBDdpXNZN6zmKsWxcoAu4y5DVKk5p
ws3fN6CBlRcDkoQJdMsOZUDcpgwlpCibzH6DfoGIa1/uhtnToNAkGVQdmlxTlVlW
uKperh1iLR/U8Sn9OH1zdkndRLsBY3Cl+ZxdkQ2qPE2+YUUhgbR8dIBEBRsvx6j4
bIVuEBSIck2q1Od/9CKeo+3otAHY0UvT55m7cYaDtwm/4uA1L62BwRKaY/3sHoA7
ia6qdNPjPNDqUGjcgPezbTyZavvadMH+M2NG5X1l8EB7JXv849HjqRu8qhZTyBtL
Ben63mStAgMBAAECggEAA6ffEtkWHr6HOka7FatHa+TKeUp3985BAyRlmGu4YdLo
26PqGe+N92vTVjLj9SffizWQGhsjlwo/QKoz8QT+oFE3/EjrCUJTnpoSXrwdLN1H
SUr08jhIaksQ7YAp9f4G/IUXDku8or9b9mWTo8+g6vjXII7/W5KLwtvjJFE1gImB
KRCzO7XJ7Zv8/7/BAJFBzOb5t97RUR+iZvFwlDeH0lDj12AZIgb9CIzfoRrHMFLR
ibds5PknPyrrbj974/FYpWx2GnO4MOnA5u2yVVGcg1iy+soZcE/1DXSngqe1lKVS
CT1BiHO/nKZKRM7XePZaSorGRWF4JiMZY7YDM9cgVwKBgQDdjqjJXffK+JpfZ0Uh
Q9QZ3F3vIZ4FJJxGiBTiYXQXW8d50ADaeUHWUCPJohHryByF+JInT7UsjEussxE4
8cMW8gzSPZe2tJqlzq5pg6eGuwaTxun5zKxq3DuNBbi+TZsbIuL6gqfI0IV5dSmS
wr/Lh3vYbpIGeS4qqIeopzI9LwKBgQDHFprfQg6aWOibof6jBjnTTQFfXKpvQQeS
Mz+D0Hp5eX4nHTW3wwPYlq/573nOD5mkhu1/mmXfegMkIVzP9uJmXijgvDKdckFV
MNHVHrYii1Bp+GWQN53NqX9ikD/1f8v+konM4dSRn0jquNK2LHT8KVnxcPAa+QZ3
QGHrMS8c4wKBgQCB8D0FfFrra0n+Ue61R7aJRDjDGpA2q/YLV5wH+OfBG06uHlOh
ziPSsUWL58Vi5wXzfIkbDSBQdCedrZeYMhIczvC+DOmBegKI4+Jed5w05FNDMBHh
Myybr3YtiwGCerlQ/PDpwt7sY38kcJZlQFqD332+vXpe2Ys98YE+ZHCOeQKBgQCy
8Q1op73qWwlPgW4W52yoECmwpeCGuMNuU+O9vW+nqVyLGYUD0yOs09v94JHxdTIa
oC/tpj/0en1CRz5dqcDaU72YKW+w9lXklUm0rbL1H5S6esoGswaCKNvXImJqbWBU
Qy/aWAywiqOGXXL+zLylPSGbknAtPjDilJquQ3neEwKBgQCaLVQ4erwjPlm1jF77
cytJWpzYT+v3bEKgFQfNUSGANqcL3TS+vPztXhlatZw2+CSUPhpXThcW7QYMvWIo
jRvMpx1lmh8Ygs1z15swWknJVUy1twdYKV/lyvjIn4VGdN8awBB5LhzPgDHiS0Q0
BaJssnoLm4Izls0Q87EHQ93fFw==
-----END PRIVATE KEY-----`)
	}

	// parse configuration
	conf := &Config{}
	if err := viper.Unmarshal(conf); err != nil {
		log.Fatal().Err(err).Msg("could not unmarshal configuration")
	}

	secureConf := &SecureConfig{}
	if err := viper.Unmarshal(secureConf); err != nil {
		log.Fatal().Err(err).Msg("could not unmarshal secure configuration")
	}

	// Fail closed outside local: a real deployment must provide its own secrets.
	if env != EnvLocal {
		if secureConf.JWTPrivateKeyPEM == "" {
			log.Fatal().Str("env", env).Msg("JWT_PRIVATE_KEY_PEM is required outside the local environment")
		}
		if conf.PostgresDSN == "" {
			log.Fatal().Str("env", env).Msg("POSTGRES_DSN is required outside the local environment")
		}
		if conf.RedisURI == "" {
			log.Fatal().Str("env", env).Msg("REDIS_URI is required outside the local environment")
		}
	}

	log.Info().Str("env", env).Msg("configuration loaded")

	return conf, secureConf
}
