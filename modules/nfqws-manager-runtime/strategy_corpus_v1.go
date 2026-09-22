package main

type v2StaticCorpusEntry struct {
	ID       string
	Name     string
	Source   string
	Protocol string
	Family   string
	Args     []string
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
	{
		ID: "omn1z-tls-ms-ovl1", Name: "Omn1z · multisplit midsld seqovl1", Source: "omn1z",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-ms-ovl336", Name: "Omn1z · multisplit midsld seqovl336", Source: "omn1z",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=336:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-ms-ovl681", Name: "Omn1z · multisplit midsld seqovl681", Source: "omn1z",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=681:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-md-ovl1", Name: "Omn1z · multidisorder midsld seqovl1", Source: "omn1z",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-fds-midsld-ovl1", Name: "Omn1z · fakedsplit midsld seqovl1", Source: "omn1z",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-fdd-midsld-ovl1", Name: "Omn1z · fakeddisorder midsld seqovl1", Source: "omn1z",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-fake-google-ms", Name: "Omn1z · fake google SNI + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=www.google.com", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-ack-ms", Name: "Omn1z · fake ack + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=0x00000000:tcp_ack=-66000:repeats=2", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-badsum-ms", Name: "Omn1z · fake badsum + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:badsum", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-fake-md5-ms", Name: "Omn1z · fake tcp_md5 + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tcp_md5", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-rep6-md", Name: "Omn1z · fake repeats + multidisorder", Source: "omn1z",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=6:tcp_ack=-66000", "--lua-desync=multidisorder:pos=1,midsld:tcp_ack=-66000"},
	},
	{
		ID: "omn1z-tls-fake-ttl4-ms", Name: "Omn1z · fake TTL4 + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_ttl=4:ip6_ttl=4", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-autottl-ms", Name: "Omn1z · fake autottl + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_autottl=0,3-20", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-rep2-fds", Name: "Omn1z · fake repeats + fakedsplit", Source: "omn1z",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=2", "--lua-desync=fakedsplit:pos=midsld"},
	},
	{
		ID: "omn1z-tls-fake-padencap-ms", Name: "Omn1z · fake padencap + multisplit sniext", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,padencap", "--lua-desync=multisplit:pos=1,sniext+1"},
	},
	{
		ID: "omn1z-tls-fake-google-only", Name: "Omn1z · fake google SNI only", Source: "omn1z",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=www.google.com:tcp_seq=10000"},
	},
	{
		ID: "omn1z-tls-ms-ovl-tsup", Name: "Omn1z · multisplit seqovl tcp_ts_up", Source: "omn1z",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "omn1z-tls-fake-fonts-ms", Name: "Omn1z · fake fonts.google SNI + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=fonts.google.com:tcp_seq=10000", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "omn1z-tls-fake-msft-md", Name: "Omn1z · fake microsoft SNI + multidisorder", Source: "omn1z",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=www.microsoft.com", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-md5-ts-ms", Name: "Omn1z · fake md5 ts + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tcp_md5:tcp_ts_up", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-ttl2-md", Name: "Omn1z · fake ttl2 + multidisorder", Source: "omn1z",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_ttl=2:ip6_ttl=2", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fake-2x-ms", Name: "Omn1z · two fakes + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=2", "--lua-desync=fake:blob=tls_clienthello:tcp_ack=-66000", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-tls-fds-ts", Name: "Omn1z · fakedsplit tcp_ts_up", Source: "omn1z",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "omn1z-tls-fdd-sniext", Name: "Omn1z · fakeddisorder sniext seqovl", Source: "omn1z",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-tls-hfs-sniext", Name: "Omn1z · hostfakesplit sniext", Source: "omn1z",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=host-2:seqovl=sniext+3:seqovl_pattern=tls_clienthello:badsum:tcp_md5:tcp_ts_up", "--lua-desync=hostfakesplit:tcp_md5:tcp_ts_up"},
	},
	{
		ID: "omn1z-tls-hfs-google", Name: "Omn1z · hostfakesplit google host", Source: "omn1z",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:host=www.google.com:seqovl=sniext+3:seqovl_pattern=tls_clienthello:badsum", "--lua-desync=hostfakesplit:tcp_ts_up"},
	},
	{
		ID: "omn1z-http-fake-ms", Name: "Omn1z · HTTP fake + multisplit", Source: "omn1z",
		Protocol: "http", Family: "fake+split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multisplit:pos=method+2"},
	},
	{
		ID: "omn1z-http-fake-md", Name: "Omn1z · HTTP fake + multidisorder", Source: "omn1z",
		Protocol: "http", Family: "fake+disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multidisorder:pos=method+2"},
	},
	{
		ID: "z2k-tls-hostfakesplit-badseq", Name: "z2k · hostfakesplit badseq/badsum", Source: "z2k",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:badseq:badsum:badseq_increment=0"},
	},
	{
		ID: "z2k-tls-hostfakesplit-repeats-ts", Name: "z2k · hostfakesplit repeats tcp_ts", Source: "z2k",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:repeats=4:tcp_ts=-43210:badsum"},
	},
	{
		ID: "z2k-tls-hostfakesplit-midhost", Name: "z2k · hostfakesplit midhost seqovl", Source: "z2k",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=host-2:seqovl=726:badsum:badseq:badseq_increment=0"},
	},
	{
		ID: "z2k-tls-fake-zero-repeats-seq2", Name: "z2k · fake zero repeats tcp_seq=2", Source: "z2k",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=11:tcp_seq=2"},
	},
	{
		ID: "z2k-tls-fake-zero-repeats-seq1000000", Name: "z2k · fake zero repeats tcp_seq=1000000", Source: "z2k",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=11:tcp_seq=1000000"},
	},
	{
		ID: "z2k-tls-fake-0f-badsum-badseq", Name: "z2k · fake 0f badsum badseq", Source: "z2k",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0F0F:badsum:badseq"},
	},
	{
		ID: "z2k-tls-fake-0f-md5", Name: "z2k · fake 0f tcp_md5", Source: "z2k",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0E0F:tcp_md5"},
	},
	{
		ID: "z2k-tls-ms-z2k-681", Name: "z2k · multisplit seqovl681", Source: "z2k",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1:seqovl=681:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "z2k-tls-ms-sniext-seqovl", Name: "z2k · multisplit sniext seqovl", Source: "z2k",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,sniext+1:seqovl=1"},
	},
	{
		ID: "z2k-tls-ms-multi-endsld", Name: "z2k · multisplit sld endsld", Source: "z2k",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,sld+1,endsld-2:seqovl=1"},
	},
	{
		ID: "z2k-tls-md-wide-positions", Name: "z2k · multidisorder wide positions", Source: "z2k",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=2,5,105,host+5,sld-1,endsld-5,endsld"},
	},
	{
		ID: "z2k-tls-md-full-positions", Name: "z2k · multidisorder full positions", Source: "z2k",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=1,sniext+1,host+1,midsld-2,midsld,midsld+2,endhost-1"},
	},
	{
		ID: "z2k-tls-fds-pos1-seq2", Name: "z2k · fakedsplit pos1 tcp_seq=2", Source: "z2k",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=1:tcp_seq=2"},
	},
	{
		ID: "z2k-tls-syndata-md", Name: "z2k · syndata + multidisorder", Source: "z2k",
		Protocol: "https", Family: "syndata",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=syndata:payload=tls_client_hello:dir=out:blob=syn_packet", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "z2k-tls-send-empty-md5", Name: "z2k · send empty tcp_md5", Source: "z2k",
		Protocol: "https", Family: "send",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=send:payload=empty:dir=out:repeats=2:tcp_md5"},
	},
	{
		ID: "z2k-p25b2-tls-fake-zero-ms", Name: "z2k P25B2 · fake zero + multisplit", Source: "z2k",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=3:tcp_seq=2", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "z2k-p25b2-tls-fake-zero-md", Name: "z2k P25B2 · fake zero + multidisorder", Source: "z2k",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=3:tcp_seq=2", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "z2k-p25b2-tls-fake-ttl4-ms", Name: "z2k P25B2 · fake ttl4 + multisplit", Source: "z2k",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:ip_ttl=4:ip6_ttl=4", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,sniext+1"},
	},
	{
		ID: "z2k-p25b2-tls-fake-md5-ms", Name: "z2k P25B2 · fake tcp_md5 + multisplit", Source: "z2k",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0F0F:tcp_md5", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "z2k-p25b2-tls-fds-sniext", Name: "z2k P25B2 · fakedsplit sniext", Source: "z2k",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "z2k-p25b2-tls-fdd-host", Name: "z2k P25B2 · fakeddisorder host", Source: "z2k",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=host+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "z2k-p25b2-tls-ms-host-sld", Name: "z2k P25B2 · multisplit host+sld", Source: "z2k",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=host+1,sld+1:seqovl=1"},
	},
	{
		ID: "z2k-p25b2-tls-md-sld-endsld", Name: "z2k P25B2 · multidisorder sld+endsld", Source: "z2k",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=sld+1,endsld-2,endsld:seqovl=1"},
	},
	{
		ID: "z2k-p25b2-tls-fake-ack-fdd", Name: "z2k P25B2 · fake ack + fakeddisorder", Source: "z2k",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:tcp_ack=-66000:repeats=2", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=midsld"},
	},
	{
		ID: "z2k-p25b2-tls-send-empty-badsum", Name: "z2k P25B2 · send empty badsum", Source: "z2k",
		Protocol: "https", Family: "send",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=send:payload=empty:dir=out:repeats=2:badsum"},
	},
	{
		ID: "z2k-p25b2-http-fake-zero-ms", Name: "z2k P25B2 · HTTP fake zero + multisplit", Source: "z2k",
		Protocol: "http", Family: "fake+split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:payload=http_req:dir=out:blob=0x00000000:repeats=2", "--lua-desync=multisplit:payload=http_req:dir=out:pos=method+2,host+1"},
	},
	{
		ID: "z2k-p25b2-http-fake-zero-md", Name: "z2k P25B2 · HTTP fake zero + multidisorder", Source: "z2k",
		Protocol: "http", Family: "fake+disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:payload=http_req:dir=out:blob=0x00000000:repeats=2", "--lua-desync=multidisorder:payload=http_req:dir=out:pos=method+2,host+1"},
	},
	{
		ID: "omn1z-p25b2-tls-fake-badsum-md", Name: "Omn1z P25B2 · fake badsum + multidisorder", Source: "omn1z",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:badsum", "--lua-desync=multidisorder:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-p25b2-tls-fake-md5-md", Name: "Omn1z P25B2 · fake md5 + multidisorder", Source: "omn1z",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tcp_md5", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "omn1z-p25b2-tls-ms-host-sniext", Name: "Omn1z P25B2 · multisplit host+sniext", Source: "omn1z",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=host+1,sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-p25b2-tls-md-host-sld", Name: "Omn1z P25B2 · multidisorder host+sld", Source: "omn1z",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=host+1,sld+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-p25b2-tls-fds-host", Name: "Omn1z P25B2 · fakedsplit host", Source: "omn1z",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=host+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-p25b2-tls-fdd-host", Name: "Omn1z P25B2 · fakeddisorder host", Source: "omn1z",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=host+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-p25b2-http-fake-ms-host", Name: "Omn1z P25B2 · HTTP fake + multisplit host", Source: "omn1z",
		Protocol: "http", Family: "fake+split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multisplit:pos=method+2,host+1"},
	},
	{
		ID: "omn1z-p25b2-http-fake-md-host", Name: "Omn1z P25B2 · HTTP fake + multidisorder host", Source: "omn1z",
		Protocol: "http", Family: "fake+disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multidisorder:pos=method+2,host+1"},
	},
	{
		ID: "z2k-p25b3-tls-hfs-sniext-md5", Name: "z2k P25B3 · hostfakesplit sniext md5", Source: "z2k",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=host-2:seqovl=sniext+3:seqovl_pattern=tls_clienthello:tcp_md5"},
	},
	{
		ID: "z2k-p25b3-tls-hfs-sld-badsum", Name: "z2k P25B3 · hostfakesplit sld badsum", Source: "z2k",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=sld+1:seqovl=1:seqovl_pattern=tls_clienthello:badsum"},
	},
	{
		ID: "z2k-p25b3-tls-hfs-host-ts", Name: "z2k P25B3 · hostfakesplit host tcp_ts", Source: "z2k",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=host+1:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "z2k-p25b3-tls-ms-endhost", Name: "z2k P25B3 · multisplit endhost", Source: "z2k",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=endhost-1,endhost:seqovl=1"},
	},
	{
		ID: "z2k-p25b3-tls-md-endhost", Name: "z2k P25B3 · multidisorder endhost", Source: "z2k",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=endhost-1,endhost:seqovl=1"},
	},
	{
		ID: "z2k-p25b3-tls-ms-midsld-sniext", Name: "z2k P25B3 · multisplit midsld+sniext", Source: "z2k",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=midsld-2,midsld,sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "z2k-p25b3-tls-md-host-midsld", Name: "z2k P25B3 · multidisorder host+midsld", Source: "z2k",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=host+1,midsld,midsld+2:seqovl=1"},
	},
	{
		ID: "z2k-p25b3-tls-fds-sld", Name: "z2k P25B3 · fakedsplit sld", Source: "z2k",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=sld+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "z2k-p25b3-tls-fdd-sld", Name: "z2k P25B3 · fakeddisorder sld", Source: "z2k",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=sld+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "z2k-p25b3-tls-fake-autottl-ms", Name: "z2k P25B3 · fake autottl + multisplit", Source: "z2k",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:ip_autottl=0,3-20", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "z2k-p25b3-tls-fake-repeats-fds", Name: "z2k P25B3 · fake repeats + fakedsplit", Source: "z2k",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=4", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=midsld"},
	},
	{
		ID: "z2k-p25b3-tls-syndata-ms", Name: "z2k P25B3 · syndata + multisplit", Source: "z2k",
		Protocol: "https", Family: "syndata",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=syndata:payload=tls_client_hello:dir=out:blob=syn_packet", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "omn1z-p25b3-tls-hfs-host-md5", Name: "Omn1z P25B3 · hostfakesplit host md5", Source: "omn1z",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=host+1:seqovl=1:seqovl_pattern=tls_clienthello:tcp_md5"},
	},
	{
		ID: "omn1z-p25b3-tls-hfs-sniext-badsum", Name: "Omn1z P25B3 · hostfakesplit sniext badsum", Source: "omn1z",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=sniext+1:seqovl=1:seqovl_pattern=tls_clienthello:badsum"},
	},
	{
		ID: "omn1z-p25b3-tls-ms-endhost", Name: "Omn1z P25B3 · multisplit endhost", Source: "omn1z",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=endhost-1,endhost:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-p25b3-tls-md-endhost", Name: "Omn1z P25B3 · multidisorder endhost", Source: "omn1z",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=endhost-1,endhost:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "omn1z-p25b3-tls-fake-autottl-md", Name: "Omn1z P25B3 · fake autottl + multidisorder", Source: "omn1z",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_autottl=0,3-20", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "omn1z-p25b3-tls-fake-repeats-ms", Name: "Omn1z P25B3 · fake repeats + multisplit", Source: "omn1z",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=4", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "omn1z-p25b3-http-ms-host-sld", Name: "Omn1z P25B3 · HTTP multisplit host+sld", Source: "omn1z",
		Protocol: "http", Family: "split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multisplit:pos=method+2,host+1,sld+1"},
	},
	{
		ID: "omn1z-p25b3-http-md-host-sld", Name: "Omn1z P25B3 · HTTP multidisorder host+sld", Source: "omn1z",
		Protocol: "http", Family: "disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multidisorder:pos=method+2,host+1,sld+1"},
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
