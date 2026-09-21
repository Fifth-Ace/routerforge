package main

const (
	v2CorpusOmn1zRepository = "Omn1z/nfqws2-keenetic-strategy-selector"
	v2CorpusOmn1zRef        = "bf4e810ef22ffb6671e97dc411234ba9430909c9"
	v2CorpusZ2KRepository   = "necronicle/z2k"
	v2CorpusZ2KRef          = "fca1ed5a452f2554b3dfa1ab18571cee7c505174"
)

type v2StaticCorpusEntry struct {
	ID           string
	Name         string
	Source       string
	Repository   string
	Ref          string
	UpstreamName string
	Protocol     string
	Family       string
	Args         []string
}

var v2StaticCorpusEntries = []v2StaticCorpusEntry{
	{
		ID: "omn1z-tls-ms-1", Name: "Omn1z · multisplit pos=1", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-1",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1"},
	},
	{
		ID: "omn1z-tls-ms-midsld", Name: "Omn1z · multisplit midsld", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-midsld",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-ms-midsld2", Name: "Omn1z · multisplit 2,midsld-2", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-midsld2",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=2,midsld-2"},
	},
	{
		ID: "omn1z-tls-ms-sniext", Name: "Omn1z · multisplit sniext+1", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-sniext",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,sniext+1"},
	},
	{
		ID: "omn1z-tls-ms-host", Name: "Omn1z · multisplit host+1", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-host",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,host+1"},
	},
	{
		ID: "omn1z-tls-ms-sld", Name: "Omn1z · multisplit sld+1", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-sld",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=sld+1"},
	},
	{
		ID: "omn1z-tls-md-midsld", Name: "Omn1z · multidisorder midsld", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-md-midsld",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-md-sniext", Name: "Omn1z · multidisorder sniext+1", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-md-sniext",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,sniext+1"},
	},
	{
		ID: "omn1z-tls-md-multi", Name: "Omn1z · multidisorder midsld+sniext", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-md-multi",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld,sniext+1"},
	},
	{
		ID: "omn1z-tls-fds-midsld", Name: "Omn1z · fakedsplit midsld", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-fds-midsld",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld"},
	},
	{
		ID: "omn1z-tls-fdd-midsld", Name: "Omn1z · fakeddisorder midsld", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-fdd-midsld",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld"},
	},
	{
		ID: "omn1z-tls-ms-sld2", Name: "Omn1z · multisplit sld+2", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-sld2",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=sld+2"},
	},
	{
		ID: "omn1z-tls-ms-multi3", Name: "Omn1z · multisplit multi-position", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-multi3",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,sniext+1,host+1,midsld"},
	},
	{
		ID: "omn1z-tls-ms-tsup", Name: "Omn1z · multisplit tcp_ts_up", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-tsup",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:tcp_ts_up"},
	},
	{
		ID: "omn1z-tls-ms-md5", Name: "Omn1z · multisplit tcp_md5", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-md5",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:tcp_md5"},
	},
	{
		ID: "omn1z-tls-md-host", Name: "Omn1z · multidisorder host+1", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-md-host",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,host+1"},
	},
	{
		ID: "omn1z-tls-md-tsup", Name: "Omn1z · multidisorder tcp_ts_up", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-md-tsup",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld:tcp_ts_up"},
	},
	{
		ID: "omn1z-tls-md-badsum", Name: "Omn1z · multidisorder badsum", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-md-badsum",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld:badsum"},
	},
	{
		ID: "omn1z-tls-fdd-badsum-md5", Name: "Omn1z · fakeddisorder badsum+md5", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-fdd-badsum-md5",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld:badsum:tcp_md5"},
	},
	{
		ID: "omn1z-http-methodeol", Name: "Omn1z · HTTP methodeol badsum", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-methodeol",
		Protocol: "http", Family: "http-method",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_methodeol:badsum"},
	},
	{
		ID: "omn1z-http-ms-method", Name: "Omn1z · HTTP multisplit method+2", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-method",
		Protocol: "http", Family: "split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multisplit:pos=method+2"},
	},
	{
		ID: "omn1z-http-md-method", Name: "Omn1z · HTTP multidisorder method+host", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-md-method",
		Protocol: "http", Family: "disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multidisorder:pos=method+2,host+1"},
	},
	{
		ID: "omn1z-http-ms-method-host", Name: "Omn1z · HTTP multisplit method+host", Source: "omn1z",
		Repository: v2CorpusOmn1zRepository, Ref: v2CorpusOmn1zRef, UpstreamName: "auto-ms-method-host",
		Protocol: "http", Family: "split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multisplit:pos=method+2,host+1"},
	},
	{
		ID: "z2k-quic-udplen-4", Name: "z2k · QUIC udplen +4", Source: "z2k",
		Repository: v2CorpusZ2KRepository, Ref: v2CorpusZ2KRef, UpstreamName: "quic_autocircular strategy=5",
		Protocol: "quic", Family: "udplen",
		Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=udplen:payload=quic_initial:dir=out:increment=4"},
	},
	{
		ID: "z2k-quic-udplen-8", Name: "z2k · QUIC udplen +8 pattern", Source: "z2k",
		Repository: v2CorpusZ2KRepository, Ref: v2CorpusZ2KRef, UpstreamName: "quic_autocircular strategy=6",
		Protocol: "quic", Family: "udplen",
		Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=udplen:payload=quic_initial:dir=out:increment=8:pattern=0xFEA82025"},
	},
	{
		ID: "z2k-quic-udplen-25", Name: "z2k · QUIC udplen +25", Source: "z2k",
		Repository: v2CorpusZ2KRepository, Ref: v2CorpusZ2KRef, UpstreamName: "quic_autocircular strategy=7",
		Protocol: "quic", Family: "udplen",
		Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=udplen:payload=quic_initial:dir=out:increment=25"},
	},
	{
		ID: "z2k-quic-ipfrag-send-drop", Name: "z2k · QUIC ipfrag send/drop", Source: "z2k",
		Repository: v2CorpusZ2KRepository, Ref: v2CorpusZ2KRef, UpstreamName: "quic_autocircular strategy=4",
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
