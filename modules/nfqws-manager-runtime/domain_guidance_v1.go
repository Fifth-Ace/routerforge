package main

import "strings"

// v2DomainGuidance is a user-facing interpretation of the existing diagnostic.
// It never starts another probe or treats router-side results as proof for LAN clients.
type v2DomainGuidance struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	NextAction string `json:"next_action"`
	NextLabel  string `json:"next_label"`
	Scope      string `json:"scope"`
}

func v2GuideDiagnostic(d v2DiagnosticVerdict) v2DomainGuidance {
	g := v2DomainGuidance{
		Title:      "Недостаточно данных",
		Summary:    "По одной проверке нельзя определить причину. Повтори её или проверь доступ с нужного устройства.",
		NextAction: "retry",
		NextLabel:  "Повторить проверку",
		Scope:      "Проверка с роутера: IPv4, TCP 443, TLS и HTTPS. Результат не подтверждает доступность с устройств локальной сети или работу UDP/QUIC.",
	}

	switch {
	case d.Code == "CLEAR_END_TO_END":
		g.Title = "С роутера сайт отвечает"
		g.Summary = "Проверка HTTPS завершилась успешно. Если сайт не работает на телефоне или ПК, проверь его маршрут, DNS и правила обработки трафика именно этого устройства."
		g.NextAction = "none"
		g.NextLabel = ""
	case d.Code == "DNS_RESOLUTION_FAILURE":
		g.Title = "Проблема с DNS"
		g.Summary = "Роутер не получил подходящий адрес сайта. Сначала проверь DNS и написание домена; подбор стратегии nfqws2 пока не поможет."
		g.NextAction = "dns"
		g.NextLabel = "Проверить DNS"
	case d.Code == "HTTP_RESPONSE_RESTRICTED_NOT_BLOCK_PROOF":
		g.Title = "Сервер отвечает, но вернул ошибку HTTP"
		g.Summary = "Код HTTP 4xx/5xx не доказывает DPI-блокировку. Сначала проверь страницу, доступ и ответ сервера."
		g.NextAction = "none"
		g.NextLabel = ""
	case d.Code == "HTTP_STREAM_CUTOFF_12_20K_SUSPECTED":
		g.Title = "Возможно прерывание передачи данных"
		g.Summary = "Ответ мог оборваться после 12–20 КБ. Подтверди повторным тестом и TCP16 Probe, прежде чем менять стратегию."
		g.NextAction = "retry"
		g.NextLabel = "Повторить проверку"
	case strings.HasPrefix(d.Code, "TCP_CONNECT_"):
		g.Title = "Не удалось установить TCP-соединение"
		g.Summary = "Причиной могут быть маршрут, удалённый сервер или сетевое ограничение. Одного сбоя TCP недостаточно, чтобы считать виноватым DPI."
		g.NextAction = "retry"
		g.NextLabel = "Повторить проверку"
	case d.StrategyRelevant && strings.HasPrefix(d.Code, "TLS_HANDSHAKE_"):
		g.Title = "Сбой при установлении TLS"
		g.Summary = "TCP-соединение установлено, но TLS не завершился. Имеет смысл проверить несколько nfqws2-стратегий в изолированном тесте."
		g.NextAction = "selector"
		g.NextLabel = "Подобрать стратегию"
	case d.StrategyRelevant && strings.HasPrefix(d.Code, "HTTP_"):
		g.Title = "TLS установлен, но HTTPS работает неправильно"
		g.Summary = "Проверь повторяемость сбоя и сравни с тестом стратегии; это ещё не доказательство DPI-блокировки."
		g.NextAction = "selector"
		g.NextLabel = "Подобрать стратегию"
	}
	return g
}
