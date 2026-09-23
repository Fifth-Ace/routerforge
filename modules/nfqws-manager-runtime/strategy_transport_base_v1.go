package main

func v2TransportBaseArgs(transport benchTransportProfile) []string {
	switch transport.ID {
	case benchTransportHTTPS:
		return []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}
	case benchTransportHTTP:
		return []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req"}
	case benchTransportQUIC:
		return []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial"}
	case benchTransportSTUN:
		return []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun"}
	default:
		return nil
	}
}
