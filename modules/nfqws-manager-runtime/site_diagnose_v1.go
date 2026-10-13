package main

import (
	"net/http"
	"strings"
)

// v2SiteContext explains what existing router-side measurements and the
// production-profile inventory can (and cannot) establish. No new probes.
type v2SiteContext struct {
	Title         string `json:"title"`
	Binding       string `json:"binding"`
	BindingDetail string `json:"binding_detail"`
	NetworkScope  string `json:"network_scope"`
	TCP16Detail   string `json:"tcp16_detail,omitempty"`
	NextAction    string `json:"next_action"`
}

type v2SiteDiagnoseResponse struct {
	OK      bool              `json:"ok"`
	Target  string            `json:"target"`
	Detect  v2DetectResponse  `json:"detect"`
	Inspect v2InspectResponse `json:"inspect"`
	Context v2SiteContext     `json:"context"`
}

func v2DescribeSiteContext(d v2DetectResponse, i v2InspectResponse) v2SiteContext {
	out := v2SiteContext{
		Title:        "Проверка сайта и текущих правил",
		Binding:      i.BindingVerdict,
		NetworkScope: "Проверено только исходящее IPv4/HTTPS-соединение самого роутера. Маршрут, DNS и политику отдельного устройства LAN эта проверка не подтверждает.",
		NextAction:   d.Guidance.NextAction,
	}
	switch i.BindingVerdict {
	case "ONE_PRODUCTION_PROFILE_MATCH":
		out.BindingDetail = "Найден один потенциально подходящий профиль nfqws2. Совпадение по правилам ещё не доказывает, что его NFQUEUE-обработка была задействована при тесте."
	case "MULTIPLE_PRODUCTION_PROFILE_MATCHES":
		out.BindingDetail = "Найдено несколько потенциально подходящих профилей. Проверь порядок правил и исключения перед изменением стратегии."
	case "LIST_MATCH_WITHOUT_PROFILE_BINDING":
		out.BindingDetail = "Домен или IP найден в списке, но соответствие активному профилю не подтверждено. Сначала проверь, какой профиль использует этот список."
	default:
		out.BindingDetail = "Совпадение с профилем не подтверждено. Это не доказательство отсутствия обработки: учитываются только доступные данные о конфигурации и списках."
	}
	if !i.RuntimeRunning {
		out.BindingDetail = "Служба nfqws2 остановлена. Настроенные списки и профили не означают, что пакеты сейчас обрабатываются. " + out.BindingDetail
		if out.NextAction == "selector" {
			out.NextAction = "service"
		}
	}
	if d.Diagnostic.Code == "DNS_RESOLUTION_FAILURE" {
		out.BindingDetail += " IP-списки не удалось сверить с адресом сайта, потому что DNS не дал пригодного IPv4."
	}
	if d.Cutoff16KSuspected || strings.HasPrefix(d.Diagnostic.Code, "HTTP_STREAM_CUTOFF_") {
		out.TCP16Detail = "Обрыв около 12–20 КБ только предполагается по этой попытке. Для вывода нужен повторный TCP16 Probe, который не запускается автоматически."
	}
	if out.NextAction == "selector" && i.BindingVerdict == "LIST_MATCH_WITHOUT_PROFILE_BINDING" {
		out.BindingDetail += " Перед применением найденного кандидата проверь привязку списка к профилю."
	}
	return out
}

// One request runs the existing detect path once, then inspects profile/list
// bindings using the SAME resolved IPv4. The legacy /detect and /inspect-target
// routes remain intact for existing clients.
func handleV2SiteDiagnose(w http.ResponseWriter, r *http.Request) {
	var req v2DetectRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid site diagnosis request"})
		return
	}
	target, err := v2NormalizeTarget(req.Target)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	detect := v2RunDetect(r.Context(), target)
	inspect := v2InspectTargetFromIPv4(target, detect.DestinationIPv4)
	writeJSON(w, http.StatusOK, v2SiteDiagnoseResponse{
		OK: true, Target: target, Detect: detect, Inspect: inspect,
		Context: v2DescribeSiteContext(detect, inspect),
	})
}
