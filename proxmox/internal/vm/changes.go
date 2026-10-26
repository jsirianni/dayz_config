package vm

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
)

// MergeProperty retains unknown Proxmox subfields and a device's leading value.
func MergeProperty(old string, updates map[string]string, leading string) (string, error) {
	if old == "" {
		return "", errors.New("cannot patch a missing device/property")
	}
	parts := strings.Split(old, ",")
	first := ""
	props := map[string]string{}
	for i, part := range parts {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			if i != 0 {
				return "", errors.New("malformed Proxmox property")
			}
			first = part
		} else {
			props[k] = v
		}
	}
	if leading != "" {
		if first != "" {
			first = leading
		} else { // net devices use model=MAC, CPU models use a leading value
			for _, m := range []string{"virtio", "e1000", "e1000-82540em", "e1000-82544gc", "e1000-82545em", "rtl8139", "vmxnet3", "ne2k_pci", "ne2k_isa", "pcnet", "i82551", "i82557b", "i82559er"} {
				if mac, ok := props[m]; ok {
					delete(props, m)
					props[leading] = mac
					break
				}
			}
			if _, ok := props[leading]; !ok {
				if _, ok := props["cputype"]; ok {
					props["cputype"] = leading
				} else {
					return "", errors.New("property has no recognized model")
				}
			}
		}
	}
	for k, v := range updates {
		if v == "" {
			delete(props, k)
		} else {
			props[k] = v
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := []string{}
	if first != "" {
		result = append(result, first)
	}
	for _, k := range keys {
		result = append(result, k+"="+props[k])
	}
	return strings.Join(result, ","), nil
}
func HardwareChanges(cfg pve.Config, h spec.Hardware) (url.Values, error) {
	if e := h.Validate(); e != nil {
		return nil, e
	}
	v := url.Values{}
	if h.CPU != "" {
		old := cfg.String("cpu")
		if old == "" {
			old = "kvm64"
		}
		cpu, e := MergeProperty(old, nil, h.CPU)
		if e != nil {
			return nil, e
		}
		v.Set("cpu", cpu)
	}
	if h.Cores > 0 {
		v.Set("cores", strconv.Itoa(h.Cores))
	}
	if h.Sockets > 0 {
		v.Set("sockets", strconv.Itoa(h.Sockets))
	}
	if h.MemoryMiB > 0 {
		v.Set("memory", strconv.Itoa(h.MemoryMiB))
	}
	return Diff(cfg, v), nil
}
func Diff(cfg pve.Config, v url.Values) url.Values {
	for k := range v {
		if cfg.String(k) == v.Get(k) {
			v.Del(k)
		}
	}
	return v
}
func Model(cfg string) (string, string, error) {
	for _, p := range strings.Split(cfg, ",") {
		k, v, ok := strings.Cut(p, "=")
		if ok {
			if _, e := netMAC(v); e == nil {
				return k, v, nil
			}
		}
	}
	return "", "", errors.New("network configuration missing MAC address")
}

var macPattern = regexp.MustCompile(`(?i)^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)

func netMAC(s string) (string, error) {
	if !macPattern.MatchString(s) {
		return "", errors.New("not MAC")
	}
	return strings.ToLower(s), nil
}

var sizePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([KMGT])(?:I?B)?$`)

func SizeBytes(s string) (int64, error) {
	m := sizePattern.FindStringSubmatch(strings.ToUpper(s))
	if m == nil {
		return 0, errors.New("size must be an absolute K/M/G/T size, for example 100G")
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	factor := math.Pow(1024, float64(strings.Index("KMGT", m[2])+1))
	if n <= 0 || n*factor > float64(math.MaxInt64) {
		return 0, errors.New("invalid disk size")
	}
	return int64(n * factor), nil
}
func DiskSize(cfg pve.Config, disk string) (int64, error) {
	for _, p := range strings.Split(cfg.String(disk), ",") {
		if strings.HasPrefix(p, "size=") {
			return SizeBytes(strings.TrimPrefix(p, "size="))
		}
	}
	return 0, fmt.Errorf("%s has no known attached disk size", disk)
}
