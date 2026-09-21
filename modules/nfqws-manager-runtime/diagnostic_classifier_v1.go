package main

import "strings"

type v2DiagnosticVerdict struct {
	Code             string   `json:"code"`
	Severity         string   `json:"severity"`
	FaultDomain      string   `json:"fault_domain"`
	Confidence       string   `json:"confidence"`
	StrategyRelevant bool     `json:"strategy_relevant"`
	Evidence         []string `json:"evidence"`
	SuggestedAction  string   `json:"suggested_action"`
}

func v2DiagnosticEvidence(stages map[string]v2StageResult, metrics v2HTTPMetrics) []string {
	out := []string{}
	for _, name := range []string{"dns", "tcp", "tls", "http"} {
		stage, ok := stages[name]
		if !ok || stage.State == "" || stage.State == "skipped" {
			continue
		}
		item := name + "=" + stage.State
		if strings.TrimSpace(stage.Detail) != "" {
			item += ":" + strings.TrimSpace(stage.Detail)
		}
		out = append(out, item)
	}
	if metrics.Cutoff16KSuspected {
		out = append(out, "http_stream=cutoff_12_20k_suspected")
	}
	if metrics.ResponseComplete {
		out = append(out, "http_response=complete")
	} else if metrics.ProgressProven {
		out = append(out, "http_response=progress_proven")
	}
	if metrics.HTTPStatus > 0 {
		out = append(out, "http_status="+itoaSmall(metrics.HTTPStatus))
	}
	return out
}

func itoaSmall(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

func v2DiagnosticDetail(stages map[string]v2StageResult, stage string) string {
	return strings.ToLower(strings.TrimSpace(stages[stage].Detail))
}

func v2DiagnosticFailureCode(prefix, detail string) (string, string) {
	switch {
	case strings.Contains(detail, "timeout"), strings.Contains(detail, "deadline exceeded"):
		return prefix + "_TIMEOUT", "medium"
	case strings.Contains(detail, "connection refused"), strings.Contains(detail, "refused"):
		return prefix + "_REFUSED", "high"
	case strings.Contains(detail, "connection reset"), strings.Contains(detail, "reset by peer"), strings.Contains(detail, "reset"):
		return prefix + "_RESET", "medium"
	case strings.Contains(detail, "unexpected eof"), detail == "eof", strings.Contains(detail, ": eof"):
		return prefix + "_EOF", "medium"
	case strings.Contains(detail, "alert"):
		return prefix + "_ALERT", "medium"
	default:
		return prefix + "_FAILURE", "low"
	}
}

func v2ClassifyDiagnostic(stages map[string]v2StageResult, m v2HTTPMetrics) v2DiagnosticVerdict {
	evidence := v2DiagnosticEvidence(stages, m)

	if stages["dns"].State == "fail" {
		return v2DiagnosticVerdict{
			Code: "DNS_RESOLUTION_FAILURE", Severity: "fail", FaultDomain: "dns",
			Confidence: "high", StrategyRelevant: false, Evidence: evidence,
			SuggestedAction: "Проверить DNS-путь до подбора NFQWS-стратегии.",
		}
	}

	if stages["tcp"].State == "fail" {
		code, confidence := v2DiagnosticFailureCode("TCP_CONNECT", v2DiagnosticDetail(stages, "tcp"))
		action := "Проверить маршрут, доступность адреса и удалённую сторону; TCP/TLS стратегия ещё не доказана как релевантная."
		if code == "TCP_CONNECT_RESET" {
			action = "Повторить проверку и сравнить с isolated candidate: reset может быть сетью, DPI или удалённой стороной."
		}
		return v2DiagnosticVerdict{
			Code: code, Severity: "fail", FaultDomain: "tcp_path",
			Confidence: confidence, StrategyRelevant: code == "TCP_CONNECT_RESET", Evidence: evidence,
			SuggestedAction: action,
		}
	}

	if stages["tls"].State == "fail" {
		code, confidence := v2DiagnosticFailureCode("TLS_HANDSHAKE", v2DiagnosticDetail(stages, "tls"))
		return v2DiagnosticVerdict{
			Code: code, Severity: "fail", FaultDomain: "tls_path",
			Confidence: confidence, StrategyRelevant: true, Evidence: evidence,
			SuggestedAction: "Проверить isolated TCP/TLS candidates; TCP уже установлен, а TLS handshake не завершён.",
		}
	}

	if m.Cutoff16KSuspected {
		return v2DiagnosticVerdict{
			Code: "HTTP_STREAM_CUTOFF_12_20K_SUSPECTED", Severity: "warn", FaultDomain: "stream_path",
			Confidence: "medium", StrategyRelevant: true, Evidence: evidence,
			SuggestedAction: "Подтвердить повторным прогоном и TCP16 probe; один частичный ответ не считается доказательством блокировки.",
		}
	}

	if stages["http"].State == "fail" {
		code, confidence := v2DiagnosticFailureCode("HTTP_TRANSPORT", v2DiagnosticDetail(stages, "http"))
		if strings.Contains(v2DiagnosticDetail(stages, "http"), "first byte") {
			code = "HTTP_FIRST_BYTE_FAILURE"
			confidence = "medium"
		}
		if strings.Contains(v2DiagnosticDetail(stages, "http"), "sufficient progress") {
			code = "HTTP_PROGRESS_NOT_PROVEN"
			confidence = "medium"
		}
		return v2DiagnosticVerdict{
			Code: code, Severity: "fail", FaultDomain: "http_path",
			Confidence: confidence, StrategyRelevant: true, Evidence: evidence,
			SuggestedAction: "Сравнить повторные baseline/candidate прогоны; TLS уже прошёл, но HTTP transport не доказал рабочий поток.",
		}
	}

	if stages["http"].State == "warn" {
		return v2DiagnosticVerdict{
			Code: "HTTP_RESPONSE_RESTRICTED_NOT_BLOCK_PROOF", Severity: "warn", FaultDomain: "application",
			Confidence: "high", StrategyRelevant: false, Evidence: evidence,
			SuggestedAction: "HTTP 4xx/5xx сам по себе не доказывает DPI-блокировку; не подбирать стратегию только по статус-коду.",
		}
	}

	if stages["http"].State == "pass" {
		return v2DiagnosticVerdict{
			Code: "CLEAR_END_TO_END", Severity: "ok", FaultDomain: "none",
			Confidence: "high", StrategyRelevant: false, Evidence: evidence,
			SuggestedAction: "DNS, TCP, TLS и HTTP прошли; bypass для этой router-side проверки не требуется.",
		}
	}

	return v2DiagnosticVerdict{
		Code: "INCONCLUSIVE", Severity: "info", FaultDomain: "unknown",
		Confidence: "low", StrategyRelevant: false, Evidence: evidence,
		SuggestedAction: "Повторить проверку или собрать более сильное evidence до изменения production.",
	}
}
