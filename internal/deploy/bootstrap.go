package deploy

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

func setupRCon(c HostConfig) error {
	host, port, err := net.SplitHostPort(c.RConAddress)
	if err != nil || host != "127.0.0.1" {
		return errors.New("RCon must use 127.0.0.1:<port>")
	}
	secret, err := os.ReadFile(c.RConPasswordFile)
	if os.IsNotExist(err) {
		var b [32]byte
		if _, err = rand.Read(b[:]); err != nil {
			return err
		}
		secret = []byte(base64.RawURLEncoding.EncodeToString(b[:]))
		if err = AtomicWrite(c.RConPasswordFile, secret, 0600); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	password := strings.TrimSpace(string(secret))
	if password == "" || strings.ContainsAny(password, "\r\n\t ") {
		return errors.New("RCon secret must be a nonempty single token")
	}
	p, err := SafeJoin(c.InstallDir, "battleye/BEServer_x64.cfg")
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(raw) > 0 {
		if err = AtomicWrite(p+".dzdeploy-backup", raw, 0600); err != nil {
			return err
		}
	}
	var keep []string
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && (strings.EqualFold(fields[0], "RConPassword") || strings.EqualFold(fields[0], "RConPort") || strings.EqualFold(fields[0], "RConIP")) {
			continue
		}
		if strings.TrimSpace(line) != "" {
			keep = append(keep, strings.TrimRight(line, "\r"))
		}
	}
	keep = append(keep, "RConIP "+host, "RConPort "+port, "RConPassword "+password)
	return AtomicWrite(p, []byte(strings.Join(keep, "\r\n")+"\r\n"), 0600)
}
func LoadHost(p string) (HostConfig, error) {
	var c HostConfig
	err := ReadJSON(p, &c)
	if err == nil {
		err = c.Validate()
	}
	return c, err
}
func LoadClient(p string) (ClientConfig, error) {
	var c ClientConfig
	raw, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	if len(raw) > MaxFileBytes {
		return c, errors.New("configuration file too large")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err = decoder.Decode(&c); err != nil {
		return c, fmt.Errorf("invalid deploy.yaml: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return c, errors.New("deploy.yaml must contain exactly one YAML document")
	}
	if c.Version != Protocol {
		return c, VersionError(c.Version)
	}
	if len(c.Targets) == 0 {
		return c, errors.New("deploy.yaml contains no targets")
	}
	base, err := filepath.Abs(filepath.Dir(p))
	if err != nil {
		return c, err
	}
	for alias, e := range c.Targets {
		if _, err := TargetFor(alias); err != nil {
			return c, err
		}
		for _, ref := range []*string{&e.KnownHosts, &e.KeyFile, &e.PublicKeyFile} {
			if *ref != "" && !filepath.IsAbs(*ref) {
				*ref = filepath.Join(base, *ref)
			}
		}
		if e.Host != nil {
			if e.Host.Target != "" && e.Host.Target != alias {
				return c, errors.New("host target must match its YAML target key")
			}
			e.Host.Target = alias
			e.Host.Version = c.Version
		}
		c.Targets[alias] = e
	}
	return c, nil
}
