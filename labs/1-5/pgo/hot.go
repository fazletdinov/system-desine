package main

import (
	"strconv"
	"strings"
)

// #region hot

// Message — разобранная строка протокола «вид|чат|текст».
type Message struct {
	Kind string
	Chat int
	Body string
}

// Handler обрабатывает сообщение одного вида. Вызов через интерфейс — косвенный: компилятор не
// знает, какой метод вызовется, и не может его встроить. PGO видит по профилю, что почти всегда это
// textHandler, и добавляет проверку типа с прямым (и встроенным) вызовом.
type Handler interface {
	Handle(m Message) int
}

type textHandler struct{ maxLen int }

func (h textHandler) Handle(m Message) int {
	if len(m.Body) > h.maxLen {
		return 0
	}
	return len(m.Body) + m.Chat%7
}

type joinHandler struct{}

func (joinHandler) Handle(m Message) int { return m.Chat }

type leaveHandler struct{}

func (leaveHandler) Handle(m Message) int { return -m.Chat }

// parse разбирает строку. Функция дороже бюджета встраивания (80 узлов), поэтому без профиля
// остаётся вызовом; для горячих мест PGO поднимает бюджет до 2000 и встраивает её.
func parse(line string) (Message, bool) {
	kind, rest, ok := strings.Cut(line, "|")
	if !ok || kind == "" {
		return Message{}, false
	}
	chat, body, ok := strings.Cut(rest, "|")
	if !ok {
		return Message{}, false
	}
	id, ok := parseID(chat)
	if !ok || (kind == "text" && body == "") {
		return Message{}, false
	}
	return Message{Kind: kind, Chat: id, Body: body}, true
}

// parseID — положительное десятичное число до 9 цифр (без переполнения int32).
func parseID(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, n > 0
}

// Router направляет сообщение обработчику по виду.
type Router struct {
	handlers map[string]Handler
}

// NewRouter собирает маршрутизатор с тремя видами сообщений.
func NewRouter() *Router {
	return &Router{handlers: map[string]Handler{
		"text":  textHandler{maxLen: 4096},
		"join":  joinHandler{},
		"leave": leaveHandler{},
	}}
}

// Route разбирает строку и вызывает обработчик; -1 — строка не разобрана или вид неизвестен.
func (r *Router) Route(line string) int {
	m, ok := parse(line)
	if !ok {
		return -1
	}
	h, ok := r.handlers[m.Kind]
	if !ok {
		return -1
	}
	return h.Handle(m)
}

// #endregion hot

// Workload — поток строк, похожий на чат: 90 % текста, по 5 % входов и выходов.
func Workload(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		switch i % 20 {
		case 0:
			lines[i] = "join|" + strconv.Itoa(i%1000+1) + "|"
		case 1:
			lines[i] = "leave|" + strconv.Itoa(i%1000+1) + "|"
		default:
			lines[i] = "text|" + strconv.Itoa(i%1000+1) + "| привет, как дела? сообщение номер " + strconv.Itoa(i)
		}
	}
	return lines
}
