package main

import "sort"

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
		ID: "other-b-tls-ms-ovl1", Name: "Other strategies · multisplit midsld seqovl1", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-ms-ovl336", Name: "Other strategies · multisplit midsld seqovl336", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=336:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-ms-ovl681", Name: "Other strategies · multisplit midsld seqovl681", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=681:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-md-ovl1", Name: "Other strategies · multidisorder midsld seqovl1", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-fds-midsld-ovl1", Name: "Other strategies · fakedsplit midsld seqovl1", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-fdd-midsld-ovl1", Name: "Other strategies · fakeddisorder midsld seqovl1", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-fake-google-ms", Name: "Other strategies · fake google SNI + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=www.google.com", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-ack-ms", Name: "Other strategies · fake ack + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=0x00000000:tcp_ack=-66000:repeats=2", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-badsum-ms", Name: "Other strategies · fake badsum + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:badsum", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-fake-md5-ms", Name: "Other strategies · fake tcp_md5 + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tcp_md5", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-rep6-md", Name: "Other strategies · fake repeats + multidisorder", Source: "other",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=6:tcp_ack=-66000", "--lua-desync=multidisorder:pos=1,midsld:tcp_ack=-66000"},
	},
	{
		ID: "other-b-tls-fake-ttl4-ms", Name: "Other strategies · fake TTL4 + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_ttl=4:ip6_ttl=4", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-autottl-ms", Name: "Other strategies · fake autottl + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_autottl=0,3-20", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-rep2-fds", Name: "Other strategies · fake repeats + fakedsplit", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=2", "--lua-desync=fakedsplit:pos=midsld"},
	},
	{
		ID: "other-b-tls-fake-padencap-ms", Name: "Other strategies · fake padencap + multisplit sniext", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,padencap", "--lua-desync=multisplit:pos=1,sniext+1"},
	},
	{
		ID: "other-b-tls-fake-google-only", Name: "Other strategies · fake google SNI only", Source: "other",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=www.google.com:tcp_seq=10000"},
	},
	{
		ID: "other-b-tls-ms-ovl-tsup", Name: "Other strategies · multisplit seqovl tcp_ts_up", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "other-b-tls-fake-fonts-ms", Name: "Other strategies · fake fonts.google SNI + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=fonts.google.com:tcp_seq=10000", "--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "other-b-tls-fake-msft-md", Name: "Other strategies · fake microsoft SNI + multidisorder", Source: "other",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=www.microsoft.com", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-md5-ts-ms", Name: "Other strategies · fake md5 ts + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tcp_md5:tcp_ts_up", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-ttl2-md", Name: "Other strategies · fake ttl2 + multidisorder", Source: "other",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_ttl=2:ip6_ttl=2", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fake-2x-ms", Name: "Other strategies · two fakes + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=2", "--lua-desync=fake:blob=tls_clienthello:tcp_ack=-66000", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-tls-fds-ts", Name: "Other strategies · fakedsplit tcp_ts_up", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "other-b-tls-fdd-sniext", Name: "Other strategies · fakeddisorder sniext seqovl", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-tls-hfs-sniext", Name: "Other strategies · hostfakesplit sniext", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=host-2:seqovl=sniext+3:seqovl_pattern=tls_clienthello:badsum:tcp_md5:tcp_ts_up", "--lua-desync=hostfakesplit:tcp_md5:tcp_ts_up"},
	},
	{
		ID: "other-b-tls-hfs-google", Name: "Other strategies · hostfakesplit google host", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:host=www.google.com:seqovl=sniext+3:seqovl_pattern=tls_clienthello:badsum", "--lua-desync=hostfakesplit:tcp_ts_up"},
	},
	{
		ID: "other-b-http-fake-ms", Name: "Other strategies · HTTP fake + multisplit", Source: "other",
		Protocol: "http", Family: "fake+split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multisplit:pos=method+2"},
	},
	{
		ID: "other-b-http-fake-md", Name: "Other strategies · HTTP fake + multidisorder", Source: "other",
		Protocol: "http", Family: "fake+disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multidisorder:pos=method+2"},
	},
	{
		ID: "other-a-tls-hostfakesplit-badseq", Name: "Other strategies · hostfakesplit badseq/badsum", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:badseq:badsum:badseq_increment=0"},
	},
	{
		ID: "other-a-tls-hostfakesplit-repeats-ts", Name: "Other strategies · hostfakesplit repeats tcp_ts", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:repeats=4:tcp_ts=-43210:badsum"},
	},
	{
		ID: "other-a-tls-hostfakesplit-midhost", Name: "Other strategies · hostfakesplit midhost seqovl", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=host-2:seqovl=726:badsum:badseq:badseq_increment=0"},
	},
	{
		ID: "other-a-tls-fake-zero-repeats-seq2", Name: "Other strategies · fake zero repeats tcp_seq=2", Source: "other",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=11:tcp_seq=2"},
	},
	{
		ID: "other-a-tls-fake-zero-repeats-seq1000000", Name: "Other strategies · fake zero repeats tcp_seq=1000000", Source: "other",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=11:tcp_seq=1000000"},
	},
	{
		ID: "other-a-tls-fake-0f-badsum-badseq", Name: "Other strategies · fake 0f badsum badseq", Source: "other",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0F0F:badsum:badseq"},
	},
	{
		ID: "other-a-tls-fake-0f-md5", Name: "Other strategies · fake 0f tcp_md5", Source: "other",
		Protocol: "https", Family: "fake",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0E0F:tcp_md5"},
	},
	{
		ID: "other-a-tls-ms-other-681", Name: "Other strategies · multisplit seqovl681", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1:seqovl=681:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-a-tls-ms-sniext-seqovl", Name: "Other strategies · multisplit sniext seqovl", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,sniext+1:seqovl=1"},
	},
	{
		ID: "other-a-tls-ms-multi-endsld", Name: "Other strategies · multisplit sld endsld", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,sld+1,endsld-2:seqovl=1"},
	},
	{
		ID: "other-a-tls-md-wide-positions", Name: "Other strategies · multidisorder wide positions", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=2,5,105,host+5,sld-1,endsld-5,endsld"},
	},
	{
		ID: "other-a-tls-md-full-positions", Name: "Other strategies · multidisorder full positions", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=1,sniext+1,host+1,midsld-2,midsld,midsld+2,endhost-1"},
	},
	{
		ID: "other-a-tls-fds-pos1-seq2", Name: "Other strategies · fakedsplit pos1 tcp_seq=2", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=1:tcp_seq=2"},
	},
	{
		ID: "other-a-tls-syndata-md", Name: "Other strategies · syndata + multidisorder", Source: "other",
		Protocol: "https", Family: "syndata",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=syndata:payload=tls_client_hello:dir=out:blob=syn_packet", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "other-a-tls-send-empty-md5", Name: "Other strategies · send empty tcp_md5", Source: "other",
		Protocol: "https", Family: "send",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=send:payload=empty:dir=out:repeats=2:tcp_md5"},
	},
	{
		ID: "other-a-p25b2-tls-fake-zero-ms", Name: "Other strategies · fake zero + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=3:tcp_seq=2", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "other-a-p25b2-tls-fake-zero-md", Name: "Other strategies · fake zero + multidisorder", Source: "other",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=3:tcp_seq=2", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "other-a-p25b2-tls-fake-ttl4-ms", Name: "Other strategies · fake ttl4 + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:ip_ttl=4:ip6_ttl=4", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,sniext+1"},
	},
	{
		ID: "other-a-p25b2-tls-fake-md5-ms", Name: "Other strategies · fake tcp_md5 + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0F0F:tcp_md5", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "other-a-p25b2-tls-fds-sniext", Name: "Other strategies · fakedsplit sniext", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-a-p25b2-tls-fdd-host", Name: "Other strategies · fakeddisorder host", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=host+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-a-p25b2-tls-ms-host-sld", Name: "Other strategies · multisplit host+sld", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=host+1,sld+1:seqovl=1"},
	},
	{
		ID: "other-a-p25b2-tls-md-sld-endsld", Name: "Other strategies · multidisorder sld+endsld", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=sld+1,endsld-2,endsld:seqovl=1"},
	},
	{
		ID: "other-a-p25b2-tls-fake-ack-fdd", Name: "Other strategies · fake ack + fakeddisorder", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:tcp_ack=-66000:repeats=2", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=midsld"},
	},
	{
		ID: "other-a-p25b2-tls-send-empty-badsum", Name: "Other strategies · send empty badsum", Source: "other",
		Protocol: "https", Family: "send",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=send:payload=empty:dir=out:repeats=2:badsum"},
	},
	{
		ID: "other-a-p25b2-http-fake-zero-ms", Name: "Other strategies · HTTP fake zero + multisplit", Source: "other",
		Protocol: "http", Family: "fake+split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:payload=http_req:dir=out:blob=0x00000000:repeats=2", "--lua-desync=multisplit:payload=http_req:dir=out:pos=method+2,host+1"},
	},
	{
		ID: "other-a-p25b2-http-fake-zero-md", Name: "Other strategies · HTTP fake zero + multidisorder", Source: "other",
		Protocol: "http", Family: "fake+disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:payload=http_req:dir=out:blob=0x00000000:repeats=2", "--lua-desync=multidisorder:payload=http_req:dir=out:pos=method+2,host+1"},
	},
	{
		ID: "other-b-p25b2-tls-fake-badsum-md", Name: "Other strategies · fake badsum + multidisorder", Source: "other",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:badsum", "--lua-desync=multidisorder:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-p25b2-tls-fake-md5-md", Name: "Other strategies · fake md5 + multidisorder", Source: "other",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:tcp_md5", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "other-b-p25b2-tls-ms-host-sniext", Name: "Other strategies · multisplit host+sniext", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=host+1,sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-p25b2-tls-md-host-sld", Name: "Other strategies · multidisorder host+sld", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=host+1,sld+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-p25b2-tls-fds-host", Name: "Other strategies · fakedsplit host", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=host+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-p25b2-tls-fdd-host", Name: "Other strategies · fakeddisorder host", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=host+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-p25b2-http-fake-ms-host", Name: "Other strategies · HTTP fake + multisplit host", Source: "other",
		Protocol: "http", Family: "fake+split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multisplit:pos=method+2,host+1"},
	},
	{
		ID: "other-b-p25b2-http-fake-md-host", Name: "Other strategies · HTTP fake + multidisorder host", Source: "other",
		Protocol: "http", Family: "fake+disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multidisorder:pos=method+2,host+1"},
	},
	{
		ID: "other-a-p25b3-tls-hfs-sniext-md5", Name: "Other strategies · hostfakesplit sniext md5", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=host-2:seqovl=sniext+3:seqovl_pattern=tls_clienthello:tcp_md5"},
	},
	{
		ID: "other-a-p25b3-tls-hfs-sld-badsum", Name: "Other strategies · hostfakesplit sld badsum", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=sld+1:seqovl=1:seqovl_pattern=tls_clienthello:badsum"},
	},
	{
		ID: "other-a-p25b3-tls-hfs-host-ts", Name: "Other strategies · hostfakesplit host tcp_ts", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=host+1:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "other-a-p25b3-tls-ms-endhost", Name: "Other strategies · multisplit endhost", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=endhost-1,endhost:seqovl=1"},
	},
	{
		ID: "other-a-p25b3-tls-md-endhost", Name: "Other strategies · multidisorder endhost", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=endhost-1,endhost:seqovl=1"},
	},
	{
		ID: "other-a-p25b3-tls-ms-midsld-sniext", Name: "Other strategies · multisplit midsld+sniext", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=midsld-2,midsld,sniext+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-a-p25b3-tls-md-host-midsld", Name: "Other strategies · multidisorder host+midsld", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=host+1,midsld,midsld+2:seqovl=1"},
	},
	{
		ID: "other-a-p25b3-tls-fds-sld", Name: "Other strategies · fakedsplit sld", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=sld+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-a-p25b3-tls-fdd-sld", Name: "Other strategies · fakeddisorder sld", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=sld+1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-a-p25b3-tls-fake-autottl-ms", Name: "Other strategies · fake autottl + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:ip_autottl=0,3-20", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "other-a-p25b3-tls-fake-repeats-fds", Name: "Other strategies · fake repeats + fakedsplit", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:repeats=4", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=midsld"},
	},
	{
		ID: "other-a-p25b3-tls-syndata-ms", Name: "Other strategies · syndata + multisplit", Source: "other",
		Protocol: "https", Family: "syndata",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=syndata:payload=tls_client_hello:dir=out:blob=syn_packet", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "other-b-p25b3-tls-hfs-host-md5", Name: "Other strategies · hostfakesplit host md5", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=host+1:seqovl=1:seqovl_pattern=tls_clienthello:tcp_md5"},
	},
	{
		ID: "other-b-p25b3-tls-hfs-sniext-badsum", Name: "Other strategies · hostfakesplit sniext badsum", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=sniext+1:seqovl=1:seqovl_pattern=tls_clienthello:badsum"},
	},
	{
		ID: "other-b-p25b3-tls-ms-endhost", Name: "Other strategies · multisplit endhost", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=endhost-1,endhost:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-p25b3-tls-md-endhost", Name: "Other strategies · multidisorder endhost", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=endhost-1,endhost:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-b-p25b3-tls-fake-autottl-md", Name: "Other strategies · fake autottl + multidisorder", Source: "other",
		Protocol: "https", Family: "fake+disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_autottl=0,3-20", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "other-b-p25b3-tls-fake-repeats-ms", Name: "Other strategies · fake repeats + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:repeats=4", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-b-p25b3-http-ms-host-sld", Name: "Other strategies · HTTP multisplit host+sld", Source: "other",
		Protocol: "http", Family: "split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multisplit:pos=method+2,host+1,sld+1"},
	},
	{
		ID: "other-b-p25b3-http-md-host-sld", Name: "Other strategies · HTTP multidisorder host+sld", Source: "other",
		Protocol: "http", Family: "disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multidisorder:pos=method+2,host+1,sld+1"},
	},
	{
		ID: "other-c-p25b4-tls-hfs-midsld-badsum", Name: "Other strategies · hostfakesplit midsld badsum", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=midsld:seqovl=1:seqovl_pattern=tls_clienthello:badsum"},
	},
	{
		ID: "other-c-p25b4-tls-hfs-endhost-ts", Name: "Other strategies · hostfakesplit endhost tcp_ts", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:payload=tls_client_hello:dir=out:midhost=endhost-1:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "other-c-p25b4-tls-ms-host-endhost", Name: "Other strategies · multisplit host+endhost", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=host+1,endhost-1,endhost:seqovl=1"},
	},
	{
		ID: "other-c-p25b4-tls-md-sniext-endhost", Name: "Other strategies · multidisorder sniext+endhost", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:payload=tls_client_hello:dir=out:pos=sniext+1,endhost-1,endhost:seqovl=1"},
	},
	{
		ID: "other-c-p25b4-tls-fds-endhost", Name: "Other strategies · fakedsplit endhost", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=endhost-1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-c-p25b4-tls-fdd-endhost", Name: "Other strategies · fakeddisorder endhost", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=endhost-1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-c-p25b4-tls-fake-ttl2-ms", Name: "Other strategies · fake ttl2 + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x00000000:ip_ttl=2:ip6_ttl=2", "--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1,midsld"},
	},
	{
		ID: "other-c-p25b4-tls-fake-md5-fdd", Name: "Other strategies · fake md5 + fakeddisorder", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0F0F:tcp_md5", "--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pos=midsld"},
	},
	{
		ID: "other-c-p25b4-tls-fake-badsum-fds", Name: "Other strategies · fake badsum + fakedsplit", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:payload=tls_client_hello:dir=out:blob=0x0F0F0F0F:badsum", "--lua-desync=fakedsplit:payload=tls_client_hello:dir=out:pos=midsld"},
	},
	{
		ID: "other-c-p25b4-tls-send-empty-ts", Name: "Other strategies · send empty tcp_ts", Source: "other",
		Protocol: "https", Family: "send",
		Args: []string{"--filter-tcp=443,2053,2083,2087,2096,8443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=send:payload=empty:dir=out:repeats=2:tcp_ts_up"},
	},
	{
		ID: "other-c-p25b4-http-ms-method-sld", Name: "Other strategies · HTTP multisplit method+sld", Source: "other",
		Protocol: "http", Family: "split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multisplit:payload=http_req:dir=out:pos=method+2,host+1,sld+1"},
	},
	{
		ID: "other-c-p25b4-http-md-method-sld", Name: "Other strategies · HTTP multidisorder method+sld", Source: "other",
		Protocol: "http", Family: "disorder",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=multidisorder:payload=http_req:dir=out:pos=method+2,host+1,sld+1"},
	},
	{
		ID: "other-d-p25b4-tls-hfs-midsld-ts", Name: "Other strategies · hostfakesplit midsld tcp_ts", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=midsld:seqovl=1:seqovl_pattern=tls_clienthello:tcp_ts_up"},
	},
	{
		ID: "other-d-p25b4-tls-hfs-endhost-md5", Name: "Other strategies · hostfakesplit endhost md5", Source: "other",
		Protocol: "https", Family: "hostfakesplit",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=hostfakesplit:midhost=endhost-1:seqovl=1:seqovl_pattern=tls_clienthello:tcp_md5"},
	},
	{
		ID: "other-d-p25b4-tls-ms-host-endhost", Name: "Other strategies · multisplit host+endhost", Source: "other",
		Protocol: "https", Family: "split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=host+1,endhost-1,endhost:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-d-p25b4-tls-md-sniext-endhost", Name: "Other strategies · multidisorder sniext+endhost", Source: "other",
		Protocol: "https", Family: "disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=sniext+1,endhost-1,endhost:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-d-p25b4-tls-fds-endhost", Name: "Other strategies · fakedsplit endhost", Source: "other",
		Protocol: "https", Family: "fake-split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=endhost-1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-d-p25b4-tls-fdd-endhost", Name: "Other strategies · fakeddisorder endhost", Source: "other",
		Protocol: "https", Family: "fake-disorder",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=endhost-1:seqovl=1:seqovl_pattern=tls_clienthello"},
	},
	{
		ID: "other-d-p25b4-tls-fake-ttl2-ms", Name: "Other strategies · fake ttl2 + multisplit", Source: "other",
		Protocol: "https", Family: "fake+split",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=tls_clienthello:ip_ttl=2:ip6_ttl=2", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "other-d-p25b4-http-fake-ms-sld", Name: "Other strategies · HTTP fake + multisplit sld", Source: "other",
		Protocol: "http", Family: "fake+split",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=fake:blob=http_req", "--lua-desync=multisplit:pos=method+2,host+1,sld+1"},
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

func v2CorpusFamilyPriority(family string) int {
	switch family {
	case "hostfakesplit":
		return 0
	case "fake+split":
		return 1
	case "fake+disorder":
		return 2
	case "fake-split":
		return 3
	case "fake-disorder":
		return 4
	case "split":
		return 5
	case "disorder":
		return 6
	case "send":
		return 7
	case "syndata":
		return 8
	case "http-method":
		return 9
	case "http-case":
		return 10
	default:
		return 99
	}
}

func v2CorpusSourcePriority(source string) int {
	switch source {
	case "curated":
		return 0
	case "other":
		return 1
	default:
		return 99
	}
}

func v2CorpusCandidatesForTransport(transport benchTransportProfile) []v2StaticCorpusEntry {
	out := []v2StaticCorpusEntry{}
	for _, item := range v2StaticStrategyCorpus() {
		if item.Protocol == transport.ID {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftFamily, rightFamily := v2CorpusFamilyPriority(out[i].Family), v2CorpusFamilyPriority(out[j].Family)
		if leftFamily != rightFamily {
			return leftFamily < rightFamily
		}
		leftSource, rightSource := v2CorpusSourcePriority(out[i].Source), v2CorpusSourcePriority(out[j].Source)
		if leftSource != rightSource {
			return leftSource < rightSource
		}
		return out[i].ID < out[j].ID
	})
	return out
}
