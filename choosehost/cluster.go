package choosehost

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/dio/transit/up"
)

const ClusterName = "auto-sni-choose-host"

type Config struct {
	HeaderName string       `json:"header_name"`
	Default    string       `json:"default"`
	Hosts      []HostConfig `json:"hosts"`
}

type HostConfig struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Hostname string `json:"hostname"`
}

type endpoint struct {
	name     string
	address  string
	hostname string
}

func init() {
	up.RegisterCluster(ClusterName, factory{})
}

type factory struct{}

func (factory) Create(raw []byte) (up.ClusterConfigFactory, error) {
	cfg, err := parseConfig(raw)
	if err != nil {
		return nil, err
	}
	return configFactory{config: cfg}, nil
}

type configFactory struct {
	config parsedConfig
}

func (f configFactory) NewCluster(_ up.ClusterHandle) up.Cluster {
	return &cluster{config: f.config}
}

func (configFactory) Close() {}

type parsedConfig struct {
	headerName string
	defaultKey string
	byName     map[string]endpoint
}

type cluster struct {
	config parsedConfig
}

func (c *cluster) Init(h up.ClusterHandle) {
	specs := make([]up.HostSpec, 0, len(c.config.byName))
	for _, ep := range c.config.byName {
		specs = append(specs, up.HostSpec{
			Address:  ep.address,
			Hostname: ep.hostname,
			Weight:   1,
		})
	}

	hosts := h.AddHosts(specs)
	for _, host := range hosts {
		h.UpdateHostHealth(host, up.HostHealthy)
	}
	h.PreInitComplete()
}

func (c *cluster) NewClusterLB() up.ClusterLB {
	return &clusterLB{config: c.config}
}

func (*cluster) ServerInitialized(up.ClusterHandle) {}
func (*cluster) DrainStarted(up.ClusterHandle)      {}
func (*cluster) Close()                             {}

func (*cluster) Shutdown(_ up.ClusterHandle, done func()) {
	done()
}

type clusterLB struct {
	up.EmptyClusterLB

	config parsedConfig
}

func (lb *clusterLB) ChooseHost(h up.ClusterLBHandle, ctx up.ClusterLBContext) (up.HostPtr, *up.ClusterLBCompletion) {
	target := lb.config.defaultKey
	if value, ok := ctx.GetHeader(lb.config.headerName); ok {
		value = normalizeKey(value)
		if value != "" {
			target = value
		}
	}

	ep, ok := lb.config.byName[target]
	if !ok {
		ep = lb.config.byName[lb.config.defaultKey]
	}

	host := h.FindHostByAddress(ep.address)
	if host == nil {
		return nil, nil
	}
	return host, nil
}

func parseConfig(raw []byte) (parsedConfig, error) {
	var cfg Config
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return parsedConfig{}, fmt.Errorf("parse cluster config: %w", err)
		}
	}

	if cfg.HeaderName == "" {
		cfg.HeaderName = "x-target-host"
	}
	if len(cfg.Hosts) == 0 {
		return parsedConfig{}, fmt.Errorf("hosts must not be empty")
	}

	byName := make(map[string]endpoint, len(cfg.Hosts))
	for _, host := range cfg.Hosts {
		ep, err := parseHost(host)
		if err != nil {
			return parsedConfig{}, err
		}
		byName[ep.name] = ep
	}

	defaultKey := normalizeKey(cfg.Default)
	if defaultKey == "" {
		defaultKey = normalizeKey(cfg.Hosts[0].Name)
	}
	if _, ok := byName[defaultKey]; !ok {
		return parsedConfig{}, fmt.Errorf("default host %q is not in hosts", cfg.Default)
	}

	return parsedConfig{
		headerName: strings.ToLower(cfg.HeaderName),
		defaultKey: defaultKey,
		byName:     byName,
	}, nil
}

func parseHost(host HostConfig) (endpoint, error) {
	name := normalizeKey(host.Name)
	hostname := normalizeKey(host.Hostname)
	if name == "" {
		name = hostname
	}
	if hostname == "" {
		hostname = name
	}
	if name == "" || hostname == "" {
		return endpoint{}, fmt.Errorf("host name and hostname must not both be empty")
	}

	address := strings.TrimSpace(host.Address)
	if address == "" {
		address = net.JoinHostPort(hostname, "443")
	}
	address, err := resolveAddress(address)
	if err != nil {
		return endpoint{}, fmt.Errorf("host %q address %q must be host:port: %w", name, address, err)
	}

	return endpoint{
		name:     name,
		address:  address,
		hostname: hostname,
	}, nil
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func resolveAddress(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if net.ParseIP(host) != nil {
		return address, nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no addresses for %s", host)
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return net.JoinHostPort(ip.String(), port), nil
		}
	}
	return net.JoinHostPort(ips[0].String(), port), nil
}
