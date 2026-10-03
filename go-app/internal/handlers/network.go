package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type networkAddress struct {
	Family    string `json:"family"`
	Address   string `json:"address"`
	PrefixLen int    `json:"prefixLen"`
	Scope     string `json:"scope"`
	Label     string `json:"label"`
}

type networkInterface struct {
	Name      string           `json:"name"`
	MAC       string           `json:"mac"`
	MTU       int              `json:"mtu"`
	State     string           `json:"state"`
	DHCP4     string           `json:"dhcp4"`
	DHCP6     string           `json:"dhcp6"`
	Addresses []networkAddress `json:"addresses"`
}

type networkRoute struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Device      string `json:"device"`
	Prefsrc     string `json:"prefsrc"`
	Protocol    string `json:"protocol"`
}

type resolverInfo struct {
	Mode          string   `json:"mode"`
	ResolvConf    string   `json:"resolvConf"`
	NameServers   []string `json:"nameServers"`
	SearchDomains []string `json:"searchDomains"`
}

type networkOverviewResponse struct {
	Hostname        string             `json:"hostname"`
	FQDN            string             `json:"fqdn"`
	Domain          string             `json:"domain"`
	SearchDomains   []string           `json:"searchDomains"`
	PrimaryIPv4     string             `json:"primaryIPv4"`
	PrimaryIPv6     string             `json:"primaryIPv6"`
	DefaultGateway  string             `json:"defaultGateway"`
	DefaultDevice   string             `json:"defaultDevice"`
	Interfaces      []networkInterface `json:"interfaces"`
	Routes          []networkRoute     `json:"routes"`
	Resolver        resolverInfo       `json:"resolver"`
	ResolverManaged string             `json:"resolverManaged"`
}

type firewallStatus struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Details  string `json:"details"`
}

type openPort struct {
	Proto   string `json:"proto"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Process string `json:"process"`
	Public  bool   `json:"public"`
}

type dockerPort struct {
	HostIP        string `json:"hostIp"`
	HostPort      uint16 `json:"hostPort"`
	ContainerPort uint16 `json:"containerPort"`
	Proto         string `json:"proto"`
}

type dockerExposure struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Image   string       `json:"image"`
	Managed bool         `json:"managed"`
	Ports   []dockerPort `json:"ports"`
}

type firewallResponse struct {
	Firewall       firewallStatus   `json:"firewall"`
	OpenPorts      []openPort       `json:"openPorts"`
	HTTPOpen       bool             `json:"httpOpen"`
	HTTPSOpen      bool             `json:"httpsOpen"`
	SSHOpen        bool             `json:"sshOpen"`
	OtherPorts     []int            `json:"otherPorts"`
	DockerExposed  []dockerExposure `json:"dockerExposed"`
	LastUpdatedISO string           `json:"lastUpdated"`
}

type ingressResponse struct {
	Provider      string       `json:"provider"`
	ContainerName string       `json:"containerName"`
	Image         string       `json:"image"`
	State         string       `json:"state"`
	Status        string       `json:"status"`
	Ports         []dockerPort `json:"ports"`
	Bindings      []string     `json:"bindings"`
	Entrypoints   []string     `json:"entrypoints"`
	HTTPRedirect  string       `json:"httpRedirect"`
	HSTS          string       `json:"hsts"`
}

type diagnosticResult struct {
	Name      string `json:"name"`
	Target    string `json:"target"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Detail    string `json:"detail"`
}

type diagnosticsResponse struct {
	PublicIP string             `json:"publicIp"`
	Results  []diagnosticResult `json:"results"`
}

func (h *Handlers) NetworkOverview(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	hostname, _ := os.Hostname()
	fqdn := hostname
	if output, err := runCommand("hostname", "-f"); err == nil && output != "" {
		fqdn = output
	}
	if fqdn == "" {
		fqdn = hostname
	}

	domain := ""
	if output, err := runCommand("hostname", "-d"); err == nil {
		domain = output
	}
	if domain == "" {
		if parts := strings.Split(fqdn, "."); len(parts) > 1 {
			domain = strings.Join(parts[1:], ".")
		}
	}

	interfaces := []networkInterface{}
	routes := []networkRoute{}
	defaultGateway := ""
	defaultDevice := ""
	primaryIPv4 := ""
	primaryIPv6 := ""

	interfaces, _ = loadInterfaces()
	routes, defaultGateway, defaultDevice = loadRoutes()
	primaryIPv4, primaryIPv6 = pickPrimaryIPs(interfaces, defaultDevice)

	resolver := loadResolverInfo()

	writeJSON(w, http.StatusOK, networkOverviewResponse{
		Hostname:        hostname,
		FQDN:            fqdn,
		Domain:          domain,
		SearchDomains:   resolver.SearchDomains,
		PrimaryIPv4:     primaryIPv4,
		PrimaryIPv6:     primaryIPv6,
		DefaultGateway:  defaultGateway,
		DefaultDevice:   defaultDevice,
		Interfaces:      interfaces,
		Routes:          routes,
		Resolver:        resolver,
		ResolverManaged: resolver.Mode,
	})
}

func (h *Handlers) NetworkFirewall(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := loadFirewallStatus()
	openPorts := loadOpenPorts()
	httpOpen, httpsOpen, sshOpen, otherPorts := summarizePorts(openPorts)
	exposed := loadDockerExposures(h)

	writeJSON(w, http.StatusOK, firewallResponse{
		Firewall:       status,
		OpenPorts:      openPorts,
		HTTPOpen:       httpOpen,
		HTTPSOpen:      httpsOpen,
		SSHOpen:        sshOpen,
		OtherPorts:     otherPorts,
		DockerExposed:  exposed,
		LastUpdatedISO: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handlers) NetworkIngress(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	provider := "traefik"
	containerName := ""
	image := ""
	state := ""
	status := ""
	ports := []dockerPort{}
	bindings := []string{}

	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "docklite_traefik" || strings.Contains(c.Image, "traefik") {
			containerName = name
			image = c.Image
			state = c.State
			status = c.Status
			ports = dockerPortsFromContainer(c)
			for _, p := range ports {
				binding := fmt.Sprintf("%s:%d", p.HostIP, p.HostPort)
				bindings = append(bindings, binding)
			}
			break
		}
	}

	entrypoints := []string{}
	entrypointSet := map[string]bool{}
	for _, port := range ports {
		if port.HostPort == 80 {
			entrypointSet["web"] = true
		}
		if port.HostPort == 443 {
			entrypointSet["websecure"] = true
		}
	}
	for key := range entrypointSet {
		entrypoints = append(entrypoints, key)
	}

	writeJSON(w, http.StatusOK, ingressResponse{
		Provider:      provider,
		ContainerName: containerName,
		Image:         image,
		State:         state,
		Status:        status,
		Ports:         ports,
		Bindings:      bindings,
		Entrypoints:   entrypoints,
		HTTPRedirect:  "unknown",
		HSTS:          "unknown",
	})
}

func (h *Handlers) NetworkPublicIP(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://api.ipify.org?format=json")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch public ip")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusInternalServerError, "failed to fetch public ip")
		return
	}
	var payload struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse public ip")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ip": payload.IP})
}

func (h *Handlers) NetworkDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	results := []diagnosticResult{}

	gateway := ""
	_, gateway, _ = loadRoutes()
	if gateway != "" {
		results = append(results, pingTest("Ping gateway", gateway))
	}
	results = append(results, pingTest("Ping 1.1.1.1", "1.1.1.1"))
	results = append(results, dnsTest("Resolve example.com", "example.com"))
	results = append(results, dnsTest("Resolve cloudflare.com", "cloudflare.com"))
	results = append(results, httpTest("HTTPS request", "https://example.com"))
	results = append(results, portTest("Port check", "127.0.0.1", 80))
	results = append(results, portTest("Port check", "127.0.0.1", 443))

	publicIP := ""
	if ip, err := fetchPublicIP(); err == nil {
		publicIP = ip
	}

	writeJSON(w, http.StatusOK, diagnosticsResponse{
		PublicIP: publicIP,
		Results:  results,
	})
}

func runCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		return result, err
	}
	return result, nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

type ipLink struct {
	Ifname    string       `json:"ifname"`
	Address   string       `json:"address"`
	MTU       int          `json:"mtu"`
	OperState string       `json:"operstate"`
	Flags     []string     `json:"flags"`
	AddrInfo  []ipAddrInfo `json:"addr_info"`
}

type ipAddrInfo struct {
	Family    string `json:"family"`
	Local     string `json:"local"`
	PrefixLen int    `json:"prefixlen"`
	Scope     string `json:"scope"`
	Label     string `json:"label"`
}

func loadInterfaces() ([]networkInterface, error) {
	if !commandExists("ip") {
		return nil, fmt.Errorf("ip command not found")
	}
	output, err := runCommand("ip", "-j", "addr")
	if err != nil {
		return nil, err
	}
	var links []ipLink
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return nil, err
	}
	interfaces := make([]networkInterface, 0, len(links))
	for _, link := range links {
		if link.Ifname == "lo" {
			continue
		}
		iface := networkInterface{
			Name:  link.Ifname,
			MAC:   link.Address,
			MTU:   link.MTU,
			State: link.OperState,
			DHCP4: "unknown",
			DHCP6: "unknown",
		}
		if commandExists("networkctl") {
			dhcp4, dhcp6 := readDhcpStatus(link.Ifname)
			if dhcp4 != "" {
				iface.DHCP4 = dhcp4
			}
			if dhcp6 != "" {
				iface.DHCP6 = dhcp6
			}
		}
		for _, addr := range link.AddrInfo {
			iface.Addresses = append(iface.Addresses, networkAddress{
				Family:    addr.Family,
				Address:   addr.Local,
				PrefixLen: addr.PrefixLen,
				Scope:     addr.Scope,
				Label:     addr.Label,
			})
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

type ipRoute struct {
	Dst      string `json:"dst"`
	Gateway  string `json:"gateway"`
	Dev      string `json:"dev"`
	Prefsrc  string `json:"prefsrc"`
	Protocol string `json:"protocol"`
}

func loadRoutes() ([]networkRoute, string, string) {
	if !commandExists("ip") {
		return nil, "", ""
	}
	output, err := runCommand("ip", "-j", "route")
	if err != nil {
		return nil, "", ""
	}
	var routesRaw []ipRoute
	if err := json.Unmarshal([]byte(output), &routesRaw); err != nil {
		return nil, "", ""
	}
	defaultGateway := ""
	defaultDevice := ""
	routes := make([]networkRoute, 0, len(routesRaw))
	for _, route := range routesRaw {
		if route.Dst == "default" {
			if defaultGateway == "" {
				defaultGateway = route.Gateway
				defaultDevice = route.Dev
			}
		}
		routes = append(routes, networkRoute{
			Destination: route.Dst,
			Gateway:     route.Gateway,
			Device:      route.Dev,
			Prefsrc:     route.Prefsrc,
			Protocol:    route.Protocol,
		})
	}
	if len(routes) > 20 {
		routes = routes[:20]
	}
	return routes, defaultGateway, defaultDevice
}

func pickPrimaryIPs(interfaces []networkInterface, defaultDevice string) (string, string) {
	ipv4 := ""
	ipv6 := ""
	pick := func(iface networkInterface) {
		for _, addr := range iface.Addresses {
			if addr.Scope != "global" {
				continue
			}
			if addr.Family == "inet" && ipv4 == "" {
				ipv4 = addr.Address
			}
			if addr.Family == "inet6" && ipv6 == "" {
				ipv6 = addr.Address
			}
		}
	}
	for _, iface := range interfaces {
		if iface.Name == defaultDevice {
			pick(iface)
			break
		}
	}
	if ipv4 == "" || ipv6 == "" {
		for _, iface := range interfaces {
			pick(iface)
		}
	}
	return ipv4, ipv6
}

func readDhcpStatus(iface string) (string, string) {
	output, err := runCommand("networkctl", "status", iface, "--no-pager")
	if err != nil {
		return "", ""
	}
	lines := strings.Split(output, "\n")
	dhcp4 := "unknown"
	dhcp6 := "unknown"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "DHCP4:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "DHCP4:"))
			dhcp4 = value
		}
		if strings.HasPrefix(trimmed, "DHCP6:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "DHCP6:"))
			dhcp6 = value
		}
	}
	return dhcp4, dhcp6
}

func loadResolverInfo() resolverInfo {
	resolver := resolverInfo{Mode: "manual", ResolvConf: "/etc/resolv.conf"}
	if link, err := os.Readlink("/etc/resolv.conf"); err == nil {
		resolver.ResolvConf = link
	}

	resolvPath := "/etc/resolv.conf"
	if strings.Contains(resolver.ResolvConf, "systemd") || commandExists("resolvectl") {
		resolver.Mode = "systemd-resolved"
		if _, err := os.Stat("/run/systemd/resolve/resolv.conf"); err == nil {
			resolvPath = "/run/systemd/resolve/resolv.conf"
		}
	}

	nameservers, search := parseResolvConf(resolvPath)
	resolver.NameServers = nameservers
	resolver.SearchDomains = search
	resolver.ResolvConf = resolvPath
	return resolver
}

func parseResolvConf(path string) ([]string, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var nameservers []string
	var searchDomains []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			nameservers = append(nameservers, fields[1])
		case "search":
			searchDomains = append(searchDomains, fields[1:]...)
		case "domain":
			searchDomains = append(searchDomains, fields[1])
		}
	}
	return dedupeStrings(nameservers), dedupeStrings(searchDomains)
}

func loadFirewallStatus() firewallStatus {
	status := firewallStatus{Provider: "unknown", Status: "unknown"}
	if commandExists("ufw") {
		output, err := runCommand("ufw", "status")
		status.Provider = "ufw"
		status.Details = output
		if err == nil {
			if strings.Contains(output, "Status: active") {
				status.Status = "active"
			} else {
				status.Status = "inactive"
			}
		}
		return status
	}
	if commandExists("nft") {
		output, err := runCommand("nft", "list", "ruleset")
		status.Provider = "nftables"
		status.Details = output
		if err == nil {
			status.Status = "detected"
		}
		return status
	}
	if commandExists("iptables") {
		output, err := runCommand("iptables", "-S")
		status.Provider = "iptables"
		status.Details = output
		if err == nil {
			status.Status = "detected"
		}
		return status
	}
	return status
}

func loadOpenPorts() []openPort {
	if !commandExists("ss") {
		return nil
	}
	output, err := runCommand("ss", "-lntupH")
	if err != nil {
		return nil
	}
	lines := strings.Split(output, "\n")
	ports := []openPort{}
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 6 {
			continue
		}
		proto := fields[0]
		local := fields[4]
		process := ""
		if len(fields) >= 7 {
			process = fields[6]
		}
		host, port := splitHostPort(local)
		if port == 0 {
			continue
		}
		public := host == "0.0.0.0" || host == "::" || host == "*" || host == "[::]"
		ports = append(ports, openPort{
			Proto:   proto,
			Address: host,
			Port:    port,
			Process: process,
			Public:  public,
		})
	}
	return ports
}

func summarizePorts(ports []openPort) (bool, bool, bool, []int) {
	otherSet := map[int]bool{}
	var httpOpen, httpsOpen, sshOpen bool
	for _, port := range ports {
		if !port.Public {
			continue
		}
		switch port.Port {
		case 80:
			httpOpen = true
		case 443:
			httpsOpen = true
		case 22:
			sshOpen = true
		default:
			otherSet[port.Port] = true
		}
	}
	otherPorts := make([]int, 0, len(otherSet))
	for port := range otherSet {
		otherPorts = append(otherPorts, port)
	}
	return httpOpen, httpsOpen, sshOpen, otherPorts
}

func loadDockerExposures(h *Handlers) []dockerExposure {
	ctx, cancel := dockerContext(context.Background())
	defer cancel()

	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil
	}
	results := []dockerExposure{}
	for _, c := range containers {
		ports := dockerPortsFromContainer(c)
		if len(ports) == 0 {
			continue
		}
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		managed := false
		if c.Labels != nil && c.Labels["docklite.managed"] == "true" {
			managed = true
		}
		results = append(results, dockerExposure{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			Managed: managed,
			Ports:   ports,
		})
	}
	return results
}

func dockerPortsFromContainer(c types.Container) []dockerPort {
	ports := []dockerPort{}
	for _, port := range c.Ports {
		if port.PublicPort == 0 {
			continue
		}
		ports = append(ports, dockerPort{
			HostIP:        port.IP,
			HostPort:      port.PublicPort,
			ContainerPort: port.PrivatePort,
			Proto:         port.Type,
		})
	}
	return ports
}

func splitHostPort(value string) (string, int) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if strings.Contains(trimmed, ":") {
		idx := strings.LastIndex(trimmed, ":")
		host := trimmed[:idx]
		portStr := trimmed[idx+1:]
		port, _ := strconv.Atoi(portStr)
		if host == "" {
			host = "*"
		}
		return host, port
	}
	port, _ := strconv.Atoi(trimmed)
	return "*", port
}

func pingTest(name string, target string) diagnosticResult {
	start := time.Now()
	if !commandExists("ping") {
		return diagnosticResult{Name: name, Target: target, Status: "error", Detail: "ping not available"}
	}
	_, err := runCommand("ping", "-c", "1", "-W", "2", target)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return diagnosticResult{Name: name, Target: target, Status: "fail", LatencyMs: latency, Detail: "no response"}
	}
	return diagnosticResult{Name: name, Target: target, Status: "ok", LatencyMs: latency}
}

func dnsTest(name string, host string) diagnosticResult {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return diagnosticResult{Name: name, Target: host, Status: "fail", LatencyMs: latency, Detail: err.Error()}
	}
	return diagnosticResult{Name: name, Target: host, Status: "ok", LatencyMs: latency, Detail: strings.Join(addrs, ", ")}
}

func httpTest(name string, url string) diagnosticResult {
	client := http.Client{Timeout: 4 * time.Second}
	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return diagnosticResult{Name: name, Target: url, Status: "fail", LatencyMs: latency, Detail: err.Error()}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return diagnosticResult{Name: name, Target: url, Status: "fail", LatencyMs: latency, Detail: resp.Status}
	}
	return diagnosticResult{Name: name, Target: url, Status: "ok", LatencyMs: latency, Detail: resp.Status}
}

func portTest(name string, host string, port int) diagnosticResult {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return diagnosticResult{Name: name, Target: fmt.Sprintf("%s:%d", host, port), Status: "fail", LatencyMs: latency, Detail: err.Error()}
	}
	_ = conn.Close()
	return diagnosticResult{Name: name, Target: fmt.Sprintf("%s:%d", host, port), Status: "ok", LatencyMs: latency}
}

func fetchPublicIP() (string, error) {
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://api.ipify.org?format=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("non-200 response")
	}
	var payload struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.IP, nil
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}
