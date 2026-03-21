package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

type dialogState int

const (
	stateIdle         dialogState = iota // 0
	stateWaitingURL                      // 1
	stateWaitingDelID                    // 2
	stateWaitingStID                     // 3
)

var SendSettings = &tele.SendOptions{
	DisableWebPagePreview: true,
	ParseMode:             tele.ModeMarkdown,
}

type Bot struct {
	bot            *tele.Bot
	monitorService MonitorService
	checkService   CheckService
	allowedChatID  int64
	state          dialogState
}

func NewBot(
	bot *tele.Bot,
	chatID int64,
	monitorService MonitorService,
	checkService CheckService,
) *Bot {
	b := &Bot{
		bot:            bot,
		monitorService: monitorService,
		checkService:   checkService,
		allowedChatID:  chatID,
		state:          stateIdle,
	}
	b.registerHandlers()
	return b
}

func (b *Bot) Start() {
	log.Println("Telegram bot started")
	go b.bot.Start()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) registerHandlers() {
	b.bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Chat().ID != b.allowedChatID {
				return c.Send("Доступ запрещён.", SendSettings)
			}
			return next(c)
		}
	})

	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/list", b.handleList)
	b.bot.Handle("/add", b.handleAdd)
	b.bot.Handle("/delete", b.handleDelete)
	b.bot.Handle("/status", b.handleStatus)
	b.bot.Handle(tele.OnText, b.handleText)
}

func (b *Bot) handleStart(c tele.Context) error {
	b.state = stateIdle
	return c.Send(
		"👋 Привет! Я бот для мониторинга сайтов.\n\n"+
			"Доступные команды:\n"+
			"/list — список всех мониторов\n"+
			"/add — добавить сайт на мониторинг\n"+
			"/delete — удалить монитор\n"+
			"/status — последняя проверка сайта",
		SendSettings,
	)
}

func (b *Bot) handleList(c tele.Context) error {
	b.state = stateIdle

	monitors, err := b.monitorService.GetAll()
	if err != nil {
		return c.Send("Ошибка при получении списка мониторов.", SendSettings)
	}
	if len(monitors) == 0 {
		return c.Send("Мониторов пока нет. Добавь первый через /add", SendSettings)
	}

	var sb strings.Builder
	sb.WriteString("📋 *Список мониторов:*\n\n")

	for _, m := range monitors {
		lastCheck, err := b.checkService.GetLastByMonitorID(m.ID)
		status := "⏳ нет данных"
		if err == nil {
			if lastCheck.IsUp {
				status = fmt.Sprintf("✅ доступен (%d ms)", lastCheck.ResponseMs)
			} else {
				status = "❌ недоступен"
			}
		}

		uptime := ""
		stats, err := b.checkService.GetUptimeStats(m.ID)
		if err == nil {
			uptime = fmt.Sprintf("Uptime 24h: %.2f%%\n", stats.Uptime24h)
		}

		sb.WriteString(fmt.Sprintf(
			"*ID:* %d | `%s`\n*Статус:* %s\n%s*Интервал:* %ds\n\n",
			m.ID, m.URL, status, uptime, m.Interval,
		))
	}

	return c.Send(sb.String(), SendSettings)
}

func (b *Bot) handleAdd(c tele.Context) error {
	b.state = stateWaitingURL
	return c.Send("Введи URL сайта для мониторинга (например: https://google.com)", SendSettings)
}

func (b *Bot) handleDelete(c tele.Context) error {
	monitors, err := b.monitorService.GetAll()
	if err != nil {
		return c.Send("Ошибка при получении списка мониторов.", SendSettings)
	}
	if len(monitors) == 0 {
		return c.Send("Мониторов пока нет.", SendSettings)
	}

	var sb strings.Builder
	sb.WriteString("Выбери ID монитора для удаления:\n\n")
	for _, m := range monitors {
		sb.WriteString(fmt.Sprintf("ID: %d | %s\n", m.ID, m.URL))
	}

	b.state = stateWaitingDelID
	return c.Send(sb.String(), SendSettings)
}

func (b *Bot) handleStatus(c tele.Context) error {
	monitors, err := b.monitorService.GetAll()
	if err != nil {
		return c.Send("Ошибка при получении списка мониторов.", SendSettings)
	}
	if len(monitors) == 0 {
		return c.Send("Мониторов пока нет.", SendSettings)
	}

	var sb strings.Builder
	sb.WriteString("Выбери ID монитора:\n\n")
	for _, m := range monitors {
		sb.WriteString(fmt.Sprintf("ID: %d | %s\n", m.ID, m.URL))
	}

	b.state = stateWaitingStID
	return c.Send(sb.String(), SendSettings)
}

func (b *Bot) handleText(c tele.Context) error {
	text := strings.TrimSpace(c.Text())

	switch b.state {
	case stateWaitingURL:
		return b.handleAddURL(c, text)
	case stateWaitingDelID:
		return b.handleDeleteID(c, text)
	case stateWaitingStID:
		return b.handleStatusID(c, text)
	default:
		return c.Send("Не понимаю команду. Используй /start чтобы увидеть список команд.", SendSettings)
	}
}

func (b *Bot) handleAddURL(c tele.Context, url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return c.Send("❌ URL должен начинаться с http:// или https://\nПопробуй ещё раз:", SendSettings)
	}
	b.state = stateIdle

	monitor, err := b.monitorService.Create(url, 60, 10)
	if err != nil {
		return c.Send("Ошибка при создании монитора.", SendSettings)
	}

	return c.Send(fmt.Sprintf(
		"✅ Монитор добавлен!\nID: %d\nURL: %s\nИнтервал: %ds",
		monitor.ID, monitor.URL, monitor.Interval,
	), SendSettings)
}

func (b *Bot) handleDeleteID(c tele.Context, text string) error {
	id, err := strconv.Atoi(text)
	if err != nil {
		return c.Send("❌ Введи числовой ID монитора:", SendSettings)
	}

	b.state = stateIdle

	if err := b.monitorService.Delete(id); err != nil {
		return c.Send("❌ Монитор не найден.", SendSettings)
	}

	return c.Send(fmt.Sprintf("✅ Монитор ID %d удалён.", id), SendSettings)
}

func (b *Bot) handleStatusID(c tele.Context, text string) error {
	id, err := strconv.Atoi(text)
	if err != nil {
		return c.Send("❌ Введи числовой ID монитора:", SendSettings)
	}

	b.state = stateIdle

	check, err := b.checkService.GetLastByMonitorID(id)
	if err != nil {
		return c.Send("❌ Монитор не найден или проверок ещё не было.", SendSettings)
	}

	status := "✅ доступен"
	if !check.IsUp {
		status = "❌ недоступен"
	}

	msg := fmt.Sprintf(
		"📊 Последняя проверка:\nСтатус: %s\nВремя ответа: %d ms\nКод ответа: %d\nПроверено: %s",
		status,
		check.ResponseMs,
		check.StatusCode,
		check.CheckedAt.Format("02.01.2006 15:04:05"),
	)

	if check.Error != "" {
		msg += fmt.Sprintf("\nОшибка: %s", check.Error)
	}

	return c.Send(msg, SendSettings)
}
