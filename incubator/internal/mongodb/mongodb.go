// Package mongodb holds the MongoDB connection shared by the services that persist data.
// Each service still owns its own collections; only the client is shared.
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/vrischmann/envconfig"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Config is read from MONGO_USERNAME, MONGO_PASSWORD, MONGO_HOST, MONGO_APP_NAME and MONGO_DATABASE.
type Config struct {
	Username string
	Password string
	Host     string
	AppName  string `envconfig:"optional"`
	// Database is the database every service's collections live in.
	Database string `envconfig:"default=prod"`
}

// LoadConfig reads the MongoDB configuration from the environment.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.InitWithPrefix(cfg, "MONGO"); err != nil {
		return nil, fmt.Errorf("loading mongo config: %w", err)
	}
	return cfg, nil
}

// ConnectionURL builds the connection URL. env "dev" uses a plain mongodb:// URL,
// anything else uses mongodb+srv:// (Atlas-style DNS seed list).
func (c *Config) ConnectionURL(env string) string {
	builder := ConnectionURLBuilder{}
	return builder.Method(env).Credentials(c.Username, c.Password).Host(c.Host).App(c.AppName).Build()
}

type ConnectionURLBuilder struct {
	lines []string
}

// Method adds the scheme to be used for the URL based on the environment.
func (cb *ConnectionURLBuilder) Method(env string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, "mongodb")
	if env != "dev" {
		cb.lines = append(cb.lines, "+srv")
	}
	cb.lines = append(cb.lines, "://")
	return cb
}

// Credentials adds the username and password, escaped so special characters are safe in a URL.
func (cb *ConnectionURLBuilder) Credentials(username string, password string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, url.QueryEscape(username), ":", url.QueryEscape(password))
	return cb
}

func (cb *ConnectionURLBuilder) Host(host string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, "@", host)
	return cb
}

func (cb *ConnectionURLBuilder) App(appName string) *ConnectionURLBuilder {
	cb.lines = append(cb.lines, "/?appName=", url.QueryEscape(appName))
	return cb
}

// Build returns the encoded URL as a string.
func (cb *ConnectionURLBuilder) Build() string {
	return strings.Join(cb.lines, "")
}

// Connect opens a client for the given URL and checks the server answers a ping.
func Connect(ctx context.Context, connectionURL string) (*mongo.Client, error) {
	if connectionURL == "" {
		return nil, errors.New("missing mongodb connection url")
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(connectionURL).SetServerAPIOptions(serverAPI)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}

	return client, nil
}

// Healthy reports whether the database's server answers a ping.
func Healthy(ctx context.Context, db *mongo.Database) bool {
	if db == nil {
		return false
	}
	return db.Client().Ping(ctx, readpref.Primary()) == nil
}
