package notifier

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const DefaultConfig = "/etc/dzsa-notifier/config.yaml"

type Config struct {
	Interval       time.Duration `yaml:"interval"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	MetricsListen  string        `yaml:"metrics_listen"`
	Endpoints      []string      `yaml:"endpoints"`
}

// Load allows an absent default file only for environment-only configuration.
func Load(path string, explicit bool, lookup func(string) (string, bool)) (Config, error) {
	c := Config{Interval: 5 * time.Minute, RequestTimeout: 10 * time.Second, MetricsListen: "127.0.0.1:9090"}
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		data, readErr := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		if readErr != nil {
			return c, fmt.Errorf("read config: %w", readErr)
		}
		if len(data) > 1<<20 {
			return c, errors.New("config exceeds 1 MiB")
		}
		d := yaml.NewDecoder(bytes.NewReader(data))
		d.KnownFields(true)
		if err := d.Decode(&c); err != nil {
			return c, fmt.Errorf("decode config: %w", err)
		}
		var extra any
		if err := d.Decode(&extra); err != io.EOF {
			return c, errors.New("config must contain exactly one YAML document")
		}
	} else {
		_, envEndpoints := lookup("DZSA_NOTIFIER_ENDPOINTS")
		if explicit || !os.IsNotExist(err) || !envEndpoints {
			return c, fmt.Errorf("open config: %w", err)
		}
	}
	for _, entry := range []struct {
		name string
		dest *time.Duration
	}{{"INTERVAL", &c.Interval}, {"REQUEST_TIMEOUT", &c.RequestTimeout}} {
		if value, ok := lookup("DZSA_NOTIFIER_" + entry.name); ok {
			*entry.dest, err = time.ParseDuration(value)
			if err != nil {
				return c, fmt.Errorf("DZSA_NOTIFIER_%s: %w", entry.name, err)
			}
		}
	}
	if value, ok := lookup("DZSA_NOTIFIER_METRICS_LISTEN"); ok {
		c.MetricsListen = value
	}
	if value, ok := lookup("DZSA_NOTIFIER_ENDPOINTS"); ok {
		c.Endpoints = strings.Split(value, ",")
	}
	for i := range c.Endpoints {
		c.Endpoints[i] = strings.TrimSpace(c.Endpoints[i])
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.Interval <= 0 || c.RequestTimeout <= 0 {
		return errors.New("interval and request_timeout must be positive durations")
	}
	if len(c.Endpoints) == 0 {
		return errors.New("endpoints must not be empty")
	}
	seen := make(map[string]bool)
	for _, endpoint := range c.Endpoints {
		ap, err := netip.ParseAddrPort(endpoint)
		if err != nil || !ap.Addr().Is4() || !ap.Addr().IsGlobalUnicast() || ap.Addr().IsPrivate() || ap.Port() == 0 {
			return fmt.Errorf("endpoint %q must be public IPv4:query-port", endpoint)
		}
		if seen[ap.String()] {
			return fmt.Errorf("duplicate endpoint %q", endpoint)
		}
		seen[ap.String()] = true
	}
	_, port, err := net.SplitHostPort(c.MetricsListen)
	if err != nil {
		return fmt.Errorf("metrics_listen: %w", err)
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("metrics_listen must have a port between 1 and 65535")
	}
	return nil
}
