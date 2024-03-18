package proxmox_plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/luthermonson/go-proxmox"
	"github.com/phaus/traefik-proxmox-plugin/internal"
)

var ParserConfigLogLevelInfo string = "info"
var ParserConfigLogLevelDebug string = "debug"

type ParserConfig struct {
	ApiEndpoint string
	TokenId     string
	Token       string
	LogLevel    string
	ValidateSSL bool
}

func NewParserConfig(apiEndpoint, tokenID, token string) (ParserConfig, error) {
	if apiEndpoint == "" || tokenID == "" || token == "" {
		return ParserConfig{}, errors.New("missing mandatory values: apiEndpoint, tokenID or token")
	}
	return ParserConfig{
		ApiEndpoint: apiEndpoint,
		TokenId:     tokenID,
		Token:       token,
		LogLevel:    ParserConfigLogLevelInfo,
		ValidateSSL: true,
	}, nil
}

func NewClient(pc ParserConfig) *proxmox.Client {
	insecureHTTPClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: getTLSClientConfig(pc),
		},
	}

	client := proxmox.NewClient(fmt.Sprintf("%s/api2/json", pc.ApiEndpoint),
		proxmox.WithHTTPClient(&insecureHTTPClient),
		proxmox.WithAPIToken(pc.TokenId, pc.Token),
		proxmox.WithLogger(GetParserConfigLogLevel(pc.LogLevel)),
	)

	return client
}

func LogVersion(client *proxmox.Client, ctx context.Context) error {
	version, err := client.Version(ctx)
	if err != nil {
		return err
	}
	log.Printf("PVE Version %s", version.Release)
	return nil
}

func GetServiceMap(client *proxmox.Client, ctx context.Context) (map[string][]internal.Service, error) {

	servicesMap := make(map[string][]internal.Service)

	nodes, err := client.Nodes(ctx)
	if err != nil {
		log.Fatalf("error, scanning nodes %s", err)
	}

	for _, nodeStatus := range nodes {
		node, err := client.Node(ctx, nodeStatus.Node)
		if err != nil {
			log.Fatalf("error %s requesting node %s", err, nodeStatus.Node)
		} else {
			services, err := scanServices(client, ctx, node)
			if err != nil {
				log.Fatalf("error, scanning services %s", err)
			}
			servicesMap[nodeStatus.Node] = services
		}
	}
	return servicesMap, nil
}

func getIPsOfService(client *proxmox.Client, ctx context.Context, node *proxmox.Node, vm *proxmox.VirtualMachine) (ips []internal.IP, err error) {
	var ifs internal.ParsedAgentInterfaces
	err = client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node.Name, vm.VMID), &ifs)
	if err != nil {
		return nil, err
	}
	return ifs.GetIPs(), nil
}

func scanServices(client *proxmox.Client, ctx context.Context, node *proxmox.Node) (services []internal.Service, err error) {
	vms, err := node.VirtualMachines(ctx)
	for _, vm := range vms {
		log.Printf("scanning vm %s/%s (%d): %s", node.Name, vm.Name, vm.VMID, vm.Status)
		var config internal.ParsedConfig
		err = client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node.Name, vm.VMID), &config)
		if err != nil {
			return services, err
		}
		if vm.Status == "running" {
			service := internal.NewService(vm.VMID, vm.Name, config.GetTraefikMap())
			ips, err := getIPsOfService(client, ctx, node, vm)
			if err == nil {
				service.IPs = ips
			}
			services = append(services, service)
		}
	}
	if err != nil {
		return services, err
	}

	cts, err := node.Containers(ctx)
	for _, ct := range cts {
		log.Printf("scanning ct %s/%s (%d): %s", node.Name, ct.Name, ct.VMID, ct.Status)
		var config internal.ParsedConfig
		err = client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", node.Name, ct.VMID), &config)
		if err != nil {
			return services, err
		}
		if ct.Status == "running" {
			service := internal.NewService(ct.VMID, ct.Name, config.GetTraefikMap())
			services = append(services, service)
		}
	}
	if err != nil {
		return services, err
	}

	return services, nil
}

func GetParserConfigLogLevel(logLevel string) (logger *proxmox.LeveledLogger) {
	if logLevel == "debug" {
		return &proxmox.LeveledLogger{Level: proxmox.LevelDebug}
	}
	return &proxmox.LeveledLogger{Level: proxmox.LevelInfo}
}

func getTLSClientConfig(pc ParserConfig) (config *tls.Config) {
	return &tls.Config{
		InsecureSkipVerify: !pc.ValidateSSL,
	}
}
