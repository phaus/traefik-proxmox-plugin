package internal

import (
	"strings"

	"github.com/luthermonson/go-proxmox"
)

type ParsedConfig struct {
	Description string `json:"description,omitempty"`
}

type ParsedAgentInterfaces struct {
	Result []struct {
		IPAddresses []IP `json:"ip-addresses"`
	} `json:"result"`
}

type Node struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"node,omitempty"`
	Status string `json:"status,omitempty"`
}

type Service struct {
	ID     uint64
	Name   string
	IPs    []IP
	Config map[string]string
}

type IP struct {
	Address     string `json:"ip-address,omitempty"`
	AddressType string `json:"ip-address-type,omitempty"`
	Prefix      uint64 `json:"prefix,omitempty"`
}

func NewService(id proxmox.StringOrUint64, name string, config map[string]string) Service {
	return Service{ID: uint64(id), Name: name, Config: config, IPs: make([]IP, 0)}
}

func (pc *ParsedConfig) GetTraefikMap() map[string]string {
	m := make(map[string]string)
	lines := strings.Split(pc.Description, "\n")
	for _, l := range lines {
		parts := strings.Split(l, ":")
		if len(parts) > 1 {
			k := strings.Replace(parts[0], "\"", "", -1)
			v := strings.Replace(parts[1], "\"", "", -1)
			if strings.HasPrefix(k, "traefik") {
				m[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	return m
}

func (pai *ParsedAgentInterfaces) GetIPs() []IP {
	ips := make([]IP, 0)
	for _, r := range pai.Result {
		ips = append(ips, r.IPAddresses...)
	}
	return ips
}
