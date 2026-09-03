package main

import "time"

type UpstreamMeta struct {
	Port              uint16 `json:"port"`
	Profile           string `json:"profile"`
	Protocol          string `json:"protocol"`
	Target            string `json:"target"`
	Name              string `json:"name"`
	SNI               string `json:"sni,omitempty"`
	Domain            string `json:"domain,omitempty"`
	Interface         string `json:"interface,omitempty"`
	LinuxInterface    string `json:"linux_interface,omitempty"`
	PolicyMark        uint32 `json:"policy_mark,omitempty"`
	PolicyTable       int    `json:"policy_table,omitempty"`
	PolicyDescription string `json:"policy_description,omitempty"`
	PolicyHasDefault  bool   `json:"policy_has_default,omitempty"`
	ProfileDNSPort    uint16 `json:"profile_dns_port,omitempty"`
	ProceedMS         int    `json:"proceed_ms,omitempty"`
	TimeoutMS         int    `json:"timeout_ms,omitempty"`
}

type WindowStats struct {
	Minutes       int     `json:"minutes"`
	Requests      uint64  `json:"requests"`
	Responses     uint64  `json:"responses"`
	LateResponses uint64  `json:"late_responses"`
	Pending       uint64  `json:"pending"`
	Success       uint64  `json:"success"`
	Errors        uint64  `json:"errors"`
	Timeouts      uint64  `json:"timeouts"`
	Fallbacks     uint64  `json:"fallbacks"`
	AvgLatencyMS  float64 `json:"avg_latency_ms"`
	P95LatencyMS  float64 `json:"p95_latency_ms"`
	MaxLatencyMS  float64 `json:"max_latency_ms"`
	QualityPct    float64 `json:"quality_pct"`
	ErrorPct      float64 `json:"error_pct"`
	TimeoutPct    float64 `json:"timeout_pct"`
	FallbackPct   float64 `json:"fallback_pct"`
	QualityStatus string  `json:"quality_status"`
}

type DiagnosticView struct {
	Ran            bool       `json:"ran"`
	LastRun        *time.Time `json:"last_run,omitempty"`
	Status         string     `json:"status,omitempty"`
	Stage          string     `json:"stage,omitempty"`
	Error          string     `json:"error,omitempty"`
	TargetIP       string     `json:"target_ip,omitempty"`
	ResolveMS      float64    `json:"resolve_ms,omitempty"`
	TCPMS          float64    `json:"tcp_ms,omitempty"`
	TLSMS          float64    `json:"tls_ms,omitempty"`
	ProtocolMS     float64    `json:"protocol_ms,omitempty"`
	HTTPStatus     int        `json:"http_status,omitempty"`
	DNSRCode       string     `json:"dns_rcode,omitempty"`
	RouteScope     string     `json:"route_scope,omitempty"`
	LastTrigger    string     `json:"last_trigger,omitempty"`
	Assessment     string     `json:"assessment,omitempty"`
	RouteMode      string     `json:"route_mode,omitempty"`
	PolicyMark     uint32     `json:"policy_mark,omitempty"`
	PolicyTable    int        `json:"policy_table,omitempty"`
	LinuxInterface string     `json:"linux_interface,omitempty"`
	DefaultStatus  string     `json:"default_status,omitempty"`
	DefaultStage   string     `json:"default_stage,omitempty"`
	DefaultError   string     `json:"default_error,omitempty"`
}

type UpstreamView struct {
	UpstreamMeta
	Requests           uint64         `json:"requests"`
	Responses          uint64         `json:"responses"`
	LateResponses      uint64         `json:"late_responses"`
	UnmatchedResponses uint64         `json:"unmatched_responses"`
	NXDomain           uint64         `json:"nxdomain"`
	ServFail           uint64         `json:"servfail"`
	Refused            uint64         `json:"refused"`
	OtherErrors        uint64         `json:"other_errors"`
	Timeouts           uint64         `json:"timeouts"`
	Fallbacks          uint64         `json:"fallbacks"`
	AvgLatencyMS       float64        `json:"avg_latency_ms"`
	MaxLatencyMS       float64        `json:"max_latency_ms"`
	HealthStatus       string         `json:"health_status"`
	HealthLatencyMS    float64        `json:"health_latency_ms"`
	LastHealthCheck    time.Time      `json:"last_health_check,omitempty"`
	LastHealthError    string         `json:"last_health_error,omitempty"`
	ConsecutiveFails   int            `json:"consecutive_fails"`
	LastObserved       time.Time      `json:"last_observed,omitempty"`
	ObservedLatencyMS  float64        `json:"observed_latency_ms,omitempty"`
	LastRequest        time.Time      `json:"last_request,omitempty"`
	Active             bool           `json:"active"`
	HealthCause        string         `json:"health_cause,omitempty"`
	Stats5m            WindowStats    `json:"stats_5m"`
	Stats1h            WindowStats    `json:"stats_1h"`
	Stats24h           WindowStats    `json:"stats_24h"`
	Diagnostic         DiagnosticView `json:"diagnostic"`
}

type QualityUpstreamView struct {
	UpstreamMeta
	HealthStatus     string         `json:"health_status"`
	HealthCause      string         `json:"health_cause,omitempty"`
	HealthLatencyMS  float64        `json:"health_latency_ms"`
	LastHealthCheck  string         `json:"last_health_check,omitempty"`
	LastHealthError  string         `json:"last_health_error,omitempty"`
	ConsecutiveFails int            `json:"consecutive_fails"`
	LastRequest      string         `json:"last_request,omitempty"`
	Active           bool           `json:"active"`
	Window           WindowStats    `json:"window"`
	Diagnostic       DiagnosticView `json:"diagnostic"`
}

type FlowEvent struct {
	Time           time.Time `json:"time"`
	Profile        string    `json:"profile"`
	Protocol       string    `json:"protocol"`
	Upstream       string    `json:"upstream"`
	Port           uint16    `json:"port"`
	Domain         string    `json:"domain"`
	QType          string    `json:"qtype"`
	Transport      string    `json:"transport"`
	Fallback       bool      `json:"fallback,omitempty"`
	ClientIP       string    `json:"client_ip,omitempty"`
	ClientMAC      string    `json:"client_mac,omitempty"`
	ClientName     string    `json:"client_name,omitempty"`
	ClientHostname string    `json:"client_hostname,omitempty"`
	ClientPolicy   string    `json:"client_policy,omitempty"`
	ClientNetwork  string    `json:"client_network,omitempty"`
	ClientAccess   string    `json:"client_access,omitempty"`
	ClientSSID     string    `json:"client_ssid,omitempty"`
	ClientAP       string    `json:"client_ap,omitempty"`
}

type ClientInfo struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac,omitempty"`
	Name      string `json:"name,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	Policy    string `json:"policy,omitempty"`
	Network   string `json:"network,omitempty"`
	NetworkID string `json:"network_id,omitempty"`
	Access    string `json:"access,omitempty"`
	SSID      string `json:"ssid,omitempty"`
	AP        string `json:"ap,omitempty"`
	Port      string `json:"port,omitempty"`
	Mesh      bool   `json:"mesh,omitempty"`
	MeshCID   string `json:"mesh_cid,omitempty"`
	Active    bool   `json:"active"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count uint64 `json:"count"`
}

type ClientView struct {
	ClientInfo
	Requests           uint64          `json:"requests"`
	Matched            uint64          `json:"matched"`
	Forwarded          uint64          `json:"forwarded"`
	CacheLocal         uint64          `json:"cache_local"`
	ClientResponses    uint64          `json:"client_responses"`
	ClientErrors       uint64          `json:"client_errors"`
	ClientTimeouts     uint64          `json:"client_timeouts"`
	Errors             uint64          `json:"errors"`
	Timeouts           uint64          `json:"timeouts"`
	Fallbacks          uint64          `json:"fallbacks"`
	AvgClientLatencyMS float64         `json:"avg_client_latency_ms"`
	P95ClientLatencyMS float64         `json:"p95_client_latency_ms"`
	MaxClientLatencyMS float64         `json:"max_client_latency_ms"`
	LastSeen           time.Time       `json:"last_seen,omitempty"`
	TopDomains         []NamedCount    `json:"top_domains"`
	TopResolvers       []NamedCount    `json:"top_resolvers"`
	Route              PolicyRouteView `json:"route"`
}

type PolicyPathView struct {
	LinuxInterface    string `json:"linux_interface"`
	KeeneticInterface string `json:"keenetic_interface,omitempty"`
	Description       string `json:"description,omitempty"`
	Type              string `json:"type,omitempty"`
	Weight            int    `json:"weight,omitempty"`
}

type PolicyRouteView struct {
	Name  string           `json:"name"`
	Mode  string           `json:"mode"`
	Mark  uint32           `json:"mark,omitempty"`
	Table int              `json:"table,omitempty"`
	Paths []PolicyPathView `json:"paths"`
}

type ClientFlowEvent struct {
	Time              time.Time `json:"time"`
	CompletedAt       time.Time `json:"completed_at"`
	ClientIP          string    `json:"client_ip"`
	ClientMAC         string    `json:"client_mac,omitempty"`
	ClientName        string    `json:"client_name,omitempty"`
	ClientHostname    string    `json:"client_hostname,omitempty"`
	ClientPolicy      string    `json:"client_policy,omitempty"`
	ClientAccess      string    `json:"client_access,omitempty"`
	Domain            string    `json:"domain"`
	QType             string    `json:"qtype"`
	Transport         string    `json:"transport"`
	Outcome           string    `json:"outcome"`
	RCode             string    `json:"rcode,omitempty"`
	LatencyMS         float64   `json:"latency_ms,omitempty"`
	Resolver          string    `json:"resolver,omitempty"`
	ResolverPort      uint16    `json:"resolver_port,omitempty"`
	Fallback          bool      `json:"fallback,omitempty"`
	UpstreamRCode     string    `json:"upstream_rcode,omitempty"`
	UpstreamLatencyMS float64   `json:"upstream_latency_ms,omitempty"`
	UpstreamTimeout   bool      `json:"upstream_timeout,omitempty"`
}

type InterfaceView struct {
	Key        string       `json:"key"`
	Name       string       `json:"name"`
	Network    string       `json:"network,omitempty"`
	SSID       string       `json:"ssid,omitempty"`
	AP         string       `json:"ap,omitempty"`
	Devices    int          `json:"devices"`
	Requests   uint64       `json:"requests"`
	Errors     uint64       `json:"errors"`
	Timeouts   uint64       `json:"timeouts"`
	Fallbacks  uint64       `json:"fallbacks"`
	TopClients []NamedCount `json:"top_clients"`
}

type ErrorEvent struct {
	Time     time.Time `json:"time"`
	Profile  string    `json:"profile,omitempty"`
	Upstream string    `json:"upstream,omitempty"`
	Port     uint16    `json:"port,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Kind     string    `json:"kind"`
	Message  string    `json:"message"`
}

type ErrorBurstView struct {
	Minute   time.Time `json:"minute"`
	Profile  string    `json:"profile"`
	Upstream string    `json:"upstream"`
	Port     uint16    `json:"port"`
	Kind     string    `json:"kind"`
	Count    uint64    `json:"count"`
	Domains  []string  `json:"domains"`
}

type DomainCount struct {
	Domain string `json:"domain"`
	Count  uint64 `json:"count"`
}

type FallbackEdge struct {
	FromProfile  string `json:"from_profile"`
	FromUpstream string `json:"from_upstream"`
	FromPort     uint16 `json:"from_port"`
	ToProfile    string `json:"to_profile"`
	ToUpstream   string `json:"to_upstream"`
	ToPort       uint16 `json:"to_port"`
	Count        uint64 `json:"count"`
}

type HistoryPoint struct {
	Time          time.Time `json:"time"`
	Requests      uint64    `json:"requests"`
	Responses     uint64    `json:"responses"`
	LateResponses uint64    `json:"late_responses"`
	Fallbacks     uint64    `json:"fallbacks"`
	Errors        uint64    `json:"errors"`
	Timeouts      uint64    `json:"timeouts"`
}
