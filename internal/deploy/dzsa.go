package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const dzsaQueryURL = "https://dayzsalauncher.com/api/v1/query/"
const dzsaRetryInterval = 15 * time.Second
const dzsaResponseLimit = 1 << 20

var ErrDZSAUnconfirmed = errors.New("DZSA verification is unconfirmed")

type DZSAMod struct {
	Name            string `json:"name"`
	SteamWorkshopID uint64 `json:"steamWorkshopId"`
}

type DZSAServer struct {
	Endpoint struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	} `json:"endpoint"`
	GamePort   int       `json:"gamePort"`
	Game       string    `json:"game"`
	Name       string    `json:"name"`
	Map        string    `json:"map"`
	Players    int       `json:"players"`
	MaxPlayers int       `json:"maxPlayers"`
	Version    string    `json:"version"`
	Mods       []DZSAMod `json:"mods"`
}

// A report is recorded only after DZSA returns a matching server and mod set.
type DZSAReport struct {
	Address   string     `json:"address"`
	CheckURL  string     `json:"checkURL"`
	CheckedAt time.Time  `json:"checkedAt"`
	Server    DZSAServer `json:"server"`
}

func publicQueryAddress(address string) (netip.AddrPort, error) {
	a, err := netip.ParseAddrPort(address)
	if err != nil || !a.Addr().Is4() || !a.Addr().IsGlobalUnicast() || a.Addr().IsPrivate() || a.Port() == 0 {
		return netip.AddrPort{}, errors.New("public_query_address must be a public IPv4 address and UDP query port (IP:port)")
	}
	return a, nil
}

func CheckDZSA(ctx context.Context, address string, target Target, hostname string) (*DZSAReport, error) {
	return checkDZSA(ctx, &http.Client{Timeout: 10 * time.Second}, dzsaQueryURL, address, target, hostname)
}

func checkDZSA(ctx context.Context, client *http.Client, baseURL, address string, target Target, hostname string) (*DZSAReport, error) {
	if err := target.Validate(); err != nil {
		return nil, err
	}
	wantAddress, err := publicQueryAddress(address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+wantAddress.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dzdeploy")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DZSA check failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DZSA check returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, dzsaResponseLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read DZSA response: %w", err)
	}
	if len(raw) > dzsaResponseLimit {
		return nil, errors.New("DZSA response is too large")
	}
	var envelope struct {
		Status *int        `json:"status"`
		Error  string      `json:"error"`
		Result *DZSAServer `json:"result"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("invalid DZSA response: %w", err)
	}
	if envelope.Status == nil {
		return nil, errors.New("DZSA response has no status")
	}
	if *envelope.Status != 0 {
		return nil, fmt.Errorf("DZSA rejected the server check: %q", envelope.Error)
	}
	if envelope.Result == nil {
		return nil, errors.New("DZSA response has no server result")
	}
	server := envelope.Result
	ip, err := netip.ParseAddr(server.Endpoint.IP)
	if err != nil || ip != wantAddress.Addr() || server.Endpoint.Port != int(wantAddress.Port()) {
		return nil, errors.New("DZSA returned a different query endpoint")
	}
	if server.Game != "dayz" || server.GamePort != target.GamePort {
		return nil, errors.New("DZSA returned an unexpected game or game port")
	}
	if server.MaxPlayers < 1 || server.Players < 0 || server.Players > server.MaxPlayers {
		return nil, errors.New("DZSA returned invalid player counts")
	}
	if err = verifyServerIdentity(target, hostname, server.Name, server.Map); err != nil {
		return nil, fmt.Errorf("DZSA: %w", err)
	}
	if err = verifyDZSAMods(target.Mods, server.Mods); err != nil {
		return nil, err
	}
	return &DZSAReport{Address: wantAddress.String(), CheckURL: "https://dayzsalauncher.com/#/servercheck/" + wantAddress.String(), CheckedAt: time.Now().UTC(), Server: *server}, nil
}

func verifyDZSAMods(expected []Mod, actual []DZSAMod) error {
	want := make(map[string]bool, len(expected))
	for _, mod := range expected {
		want[mod.ID] = true
	}
	seen := make(map[string]bool, len(actual))
	var unexpected, missing []string
	for _, mod := range actual {
		id := strconv.FormatUint(mod.SteamWorkshopID, 10)
		if mod.SteamWorkshopID == 0 || seen[id] {
			return fmt.Errorf("DZSA returned an invalid or duplicate Workshop ID %s", id)
		}
		seen[id] = true
		if !want[id] {
			unexpected = append(unexpected, id)
		}
	}
	for id := range want {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	if len(missing)+len(unexpected) > 0 {
		return fmt.Errorf("DZSA mod mismatch: missing Workshop IDs %v; unexpected Workshop IDs %v", missing, unexpected)
	}
	return nil
}

func configuredHostname(raw []byte) (string, error) {
	match := hostnamePattern.FindSubmatch(raw)
	if len(match) != 2 {
		return "", errors.New("deployed server config has no hostname")
	}
	return string(match[1]), nil
}

func verifyServerIdentity(target Target, expectedName, name, terrain string) error {
	parts := strings.Split(target.Mission, ".")
	if !strings.EqualFold(terrain, parts[len(parts)-1]) {
		return fmt.Errorf("query reports unexpected map %q", terrain)
	}
	// Steam's k_cbMaxGameServerName is 64 UTF-8 bytes including the NUL.
	// DayZ can retain a longer configured hostname while advertising 63 bytes.
	const steamHostnameBytes = 63
	matches := name == expectedName
	if len(expectedName) > steamHostnameBytes && name == expectedName[:steamHostnameBytes] {
		matches = true
	}
	if expectedName == "" || !matches {
		return fmt.Errorf("query does not match deployed hostname: got %q, expected %q", name, expectedName)
	}
	return nil
}

// CheckDZSA uses the installed manifest and live config; it never submits a job.
func (r *Remote) CheckDZSA(ctx context.Context, alias string) (*DZSAReport, error) {
	return r.checkDZSA(ctx, alias, &http.Client{Timeout: 10 * time.Second}, dzsaQueryURL)
}

func (r *Remote) checkDZSA(ctx context.Context, alias string, client *http.Client, baseURL string) (*DZSAReport, error) {
	state, err := r.State(alias)
	if err != nil {
		return nil, err
	}
	if state.Installed == nil {
		return nil, errors.New("deploy configuration before checking DZSA")
	}
	var host HostConfig
	if err = r.ReadJSON("host.json", &host); err != nil {
		return nil, err
	}
	if host.Target != alias {
		return nil, errors.New("host config target mismatch")
	}
	if err = host.Validate(); err != nil {
		return nil, err
	}
	if err = checkOwnership(*state.Installed, alias); err != nil {
		return nil, err
	}
	address := host.PublicQueryAddress
	if r.Endpoint.Host != nil && r.Endpoint.Host.PublicQueryAddress != "" {
		address = r.Endpoint.Host.PublicQueryAddress
	}
	if address == "" {
		sshHost, _, err := net.SplitHostPort(r.Endpoint.Address)
		if err != nil {
			return nil, err
		}
		address = net.JoinHostPort(sshHost, strconv.Itoa(state.Installed.Launcher.QueryPort))
		if _, err = publicQueryAddress(address); err != nil {
			return nil, errors.New("set host.public_query_address in deploy.yaml to the existing public IP:query port for DZSA")
		}
	}
	root := "/" + strings.TrimPrefix(strings.ReplaceAll(host.InstallDir, "\\", "/"), "/")
	file, err := r.SFTP.Open(path.Join(root, "serverDZ.cfg"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil || len(raw) > MaxFileBytes {
		return nil, errors.New("cannot read bounded deployed server config")
	}
	hostname, err := configuredHostname(raw)
	if err != nil {
		return nil, err
	}
	return checkDZSA(ctx, client, baseURL, address, state.Installed.Launcher, hostname)
}

func (g *NativeGame) checkDZSA(ctx context.Context, target Target, hostname string) (*DZSAReport, error) {
	if g.Config.PublicQueryAddress == "" {
		return nil, nil
	}
	if time.Now().Before(g.nextDZSA) {
		return g.lastDZSAReport, g.lastDZSAError
	}
	g.nextDZSA = time.Now().Add(dzsaRetryInterval)
	report, err := CheckDZSA(ctx, g.Config.PublicQueryAddress, target, hostname)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrDZSAUnconfirmed, err)
	}
	g.lastDZSAReport = report
	g.lastDZSAError = err
	return report, err
}

// Keep file reads here separate from RCon, which remains a shutdown channel.
func (g *NativeGame) expectedHostname() (string, error) {
	raw, err := os.ReadFile(filepath.Join(g.Config.InstallDir, "serverDZ.cfg"))
	if err != nil {
		return "", err
	}
	return configuredHostname(raw)
}
