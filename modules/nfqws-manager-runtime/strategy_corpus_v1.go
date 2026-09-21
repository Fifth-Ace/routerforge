package main

type v2StaticCorpusEntry struct {
	ID           string
	Name         string
	Source       string
	Protocol     string
	Family       string
	Args         []string
}

var v2StaticCorpusEntries = []v2StaticCorpusEntry{
	{
		ID: "rf-tls-ms-1", Name: "RouterForge · multisplit pos=1", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1"},
	},
	{
		ID: "rf-tls-ms-midsld", Name: "RouterForge · multisplit midsld", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "rf-tls-ms-midsld2", Name: "RouterForge · multisplit 2,midsld-2", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=2,midsld-2"},
	},
	{
		ID: "rf-tls-ms-sniext", Name: "RouterForge · multisplit sniext+1", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,sniext+1"},
	},
	{
		ID: "rf-tls-ms-host", Name: "RouterForge · multisplit host+1", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,host+1"},
	},
	{
		ID: "rf-tls-ms-sld", Name: "RouterForge · multisplit sld+1", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=sld+1"},
	},
	{
		ID: "rf-tls-md-midsld", Name: "RouterForge · multidisorder midsld", Source: "curated",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "rf-tls-md-sniext", Name: "RouterForge · multidisorder sniext+1", Source: "curated",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,sniext+1"},
	},
	{
		ID: "rf-tls-md-multi", Name: "RouterForge · multidisorder midsld+sniext", Source: "curated",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld,sniext+1"},
	},
	{
		ID: "rf-tls-fds-midsld", Name: "RouterForge · fakedsplit midsld", Source: "curated",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld"},
	},
	{
		ID: "rf-tls-fdd-midsld", Name: "RouterForge · fakeddisorder midsld", Source: "curated",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld"},
	},
	{
		ID: "rf-tls-ms-sld2", Name: "RouterForge · multisplit sld+2", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=sld+2"},
	},
	{
		ID: "rf-tls-ms-multi3", Name: "RouterForge · multisplit multi-position", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,sniext+1,host+1,midsld"},
	},
	{
		ID: "rf-tls-ms-tsup", Name: "RouterForge · multisplit tcp_ts_up", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:tcp_ts_up"},
	},
	{
		ID: "rf-tls-ms-md5", Name: "RouterForge · multisplit tcp_md5", Source: "curated",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:tcp_md5"},
	},
	{
		ID: "rf-tls-md-host", Name: "RouterForge · multidisorder host+1", Source: "curated",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,host+1"},
	},
	{
		ID: "rf-tls-md-tsup", Name: "RouterForge · multidisorder tcp_ts_up", Source: "curated",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld:tcp_ts_up"},
	},
	{
		ID: "rf-tls-md-badsum", Name: "RouterForge · multidisorder badsum", Source: "curated",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld:badsum"},
	},
	{
		ID: "rf-tls-fdd-badsum-md5", Name: "RouterForge · fakeddisorder badsum+md5", Source: "curated",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld:badsum:tcp_md5"},
	},
	{
		ID: "rf-http-methodeol", Name: "RouterForge · HTTP methodeol badsum", Source: "curated",
		Protocol: "http", Family: "http-method",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_methodeol:badsum"},
	},
	{
		ID: "rf-http-ms-method", Name: "RouterForge · HTTP multisplit method+2", Source: "curated",
		Protocol: "http", Family: "split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multisplit:pos=method+2"},
	},
	{
		ID: "rf-http-md-method", Name: "RouterForge · HTTP multidisorder method+host", Source: "curated",
		Protocol: "http", Family: "disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multidisorder:pos=method+2,host+1"},
	},
	{
		ID: "rf-http-ms-method-host", Name: "RouterForge · HTTP multisplit method+host", Source: "curated",
		Protocol: "http", Family: "split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multisplit:pos=method+2,host+1"},
	},
	{
		ID: "rf-quic-udplen-4", Name: "RouterForge · QUIC udplen +4", Source: "curated",
		Protocol: "quic", Family: "udplen",
		Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=udplen:payload=quic_initial:dir=out:increment=4"},
	},
	{
		ID: "rf-quic-udplen-8", Name: "RouterForge · QUIC udplen +8 pattern", Source: "curated",
		Protocol: "quic", Family: "udplen",
		Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=udplen:payload=quic_initial:dir=out:increment=8:pattern=0xFEA82025"},
	},
	{
		ID: "rf-quic-udplen-25", Name: "RouterForge · QUIC udplen +25", Source: "curated",
		Protocol: "quic", Family: "udplen",
		Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=udplen:payload=quic_initial:dir=out:increment=25"},
	},
	{
		ID: "rf-quic-ipfrag-send-drop", Name: "RouterForge · QUIC ipfrag send/drop", Source: "curated",
		Protocol: "quic", Family: "ipfrag",
		Args: []string{
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial",
			"--lua-desync=send:payload=quic_initial:dir=out:ipfrag:ipfrag_pos_udp=8:out_range=-n3",
			"--lua-desync=drop:payload=quic_initial:dir=out:out_range=-n3",
		},
	},
}

func v2StaticStrategyCorpus() []v2StaticCorpusEntry {
	out := make([]v2StaticCorpusEntry, len(v2StaticCorpusEntries))
	copy(out, v2StaticCorpusEntries)
	for i := range out {
		out[i].Args = append([]string{}, out[i].Args...)
	}
	return out
}

func v2CorpusCandidatesForTransport(transport benchTransportProfile) []v2StaticCorpusEntry {
	out := []v2StaticCorpusEntry{}
	for _, item := range v2StaticStrategyCorpus() {
		if item.Protocol == transport.ID {
			out = append(out, item)
		}
	}
	return out
}
