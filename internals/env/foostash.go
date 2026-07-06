// Package env provides environment secret providers for mgr.
package env

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	foostash "github.com/Omotolani98/foostash-go-sdk"
	"github.com/Omotolani98/mgr/internals/config"
)

type Provider interface {
	Get(context.Context, string) (string, error)
	Pull(context.Context) (map[string]string, error)
	Watch(context.Context, time.Duration, ...foostash.WatchOption) (<-chan foostash.Snapshot, error)
}

type FoostashOptions struct {
	ServerURL    string
	Project      string
	Env          string
	SSHKeyPath   string
	MasterKey    string
	MasterKeyEnv string
}

func OptionsFromConfig(cfg config.FoostashConfig) FoostashOptions {
	return FoostashOptions{
		ServerURL:    cfg.ServerURL,
		Project:      cfg.Project,
		Env:          cfg.Env,
		SSHKeyPath:   cfg.SSHKeyPath,
		MasterKey:    cfg.MasterKey,
		MasterKeyEnv: cfg.MasterKeyEnv,
	}
}

func (o FoostashOptions) ResolveMasterKey() string {
	if o.MasterKey != "" {
		return o.MasterKey
	}
	envName := o.MasterKeyEnv
	if envName == "" {
		envName = "FOOSTASH_MASTER_KEY"
	}
	return os.Getenv(envName)
}

func NewFoostashProvider(opts FoostashOptions) (Provider, error) {
	masterKey := opts.ResolveMasterKey()
	if masterKey == "" {
		envName := opts.MasterKeyEnv
		if envName == "" {
			envName = "FOOSTASH_MASTER_KEY"
		}
		return nil, fmt.Errorf("foostash master key is required; set %s or configure foostash.master_key", envName)
	}
	client, err := foostash.New(foostash.Config{
		ServerURL:  opts.ServerURL,
		Project:    opts.Project,
		Env:        opts.Env,
		SSHKeyPath: opts.SSHKeyPath,
		MasterKey:  masterKey,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func RenderDotenv(secrets map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(secrets))
	for key := range secrets {
		if !validEnvKey(key) {
			return nil, fmt.Errorf("invalid env key %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		value := secrets[key]
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("secret %s contains a newline; dotenv rendering is not safe", key)
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func Status(ctx context.Context, provider Provider) (int, error) {
	secrets, err := provider.Pull(ctx)
	if err != nil {
		return 0, err
	}
	return len(secrets), nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if r == '_' || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9' && i > 0) {
			continue
		}
		return false
	}
	return true
}

func IsAuthError(err error) bool {
	return errors.Is(err, foostash.ErrUnauthorized) || errors.Is(err, foostash.ErrForbidden)
}
