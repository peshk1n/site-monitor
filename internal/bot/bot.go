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
	stateIdle            dialogState = iota // 0
	stateWaitingURL                         // 1
	stateWaitingDelID                       // 2
	stateWaitingStID                        // 3
	stateWaitingToggleID                    // 4
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
				return c.Send("❌ *Access denied.*", SendSettings)
			}
			return next(c)
		}
	})

	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/list", b.handleList)
	b.bot.Handle("/add", b.handleAdd)
	b.bot.Handle("/delete", b.handleDelete)
	b.bot.Handle("/toggle", b.handleToggle)
	b.bot.Handle("/status", b.handleStatus)
	b.bot.Handle(tele.OnText, b.handleText)
}

func (b *Bot) handleStart(c tele.Context) error {
	b.state = stateIdle
	return c.Send(
		"👋 *Hello! I'm a website monitoring bot.*\n\n"+
			"*Available commands:*\n"+
			"/list — show all monitors\n"+
			"/add — add a new monitor\n"+
			"/delete — delete a monitor\n"+
			"/toggle — enable/disable a monitor\n"+
			"/status — show last check result",
		SendSettings,
	)
}

func (b *Bot) handleList(c tele.Context) error {
	b.state = stateIdle
	monitors, err := b.monitorService.GetAll()
	if err != nil {
		return c.Send("❌ *Failed to retrieve monitor list.*", SendSettings)
	}
	if len(monitors) == 0 {
		return c.Send("ℹ️ *No monitors yet. Add one using /add*", SendSettings)
	}

	var sb strings.Builder
	sb.WriteString("📋 *Monitor List:*\n\n")

	for _, m := range monitors {
		var status string

		if !m.IsActive {
			status = "🟡 paused"
		} else {
			lastCheck, err := b.checkService.GetLastByMonitorID(m.ID)
			if err != nil {
				status = "⚪ unknown"
			} else if lastCheck.IsUp {
				status = fmt.Sprintf("🟢 up (%d ms)", lastCheck.ResponseMs)
			} else {
				status = "🔴 down"
			}
		}

		uptime := ""
		stats, err := b.checkService.GetUptimeStats(m.ID)
		if err == nil {
			uptime = fmt.Sprintf("*Uptime (24h):* %.2f%%\n", stats.Uptime24h)
		}

		sb.WriteString(fmt.Sprintf(
			"*ID:* %d | `%s`\n*Status:* %s\n%s*Interval:* %ds\n\n",
			m.ID, m.URL, status, uptime, m.Interval,
		))
	}

	return c.Send(sb.String(), SendSettings)
}

func (b *Bot) handleAdd(c tele.Context) error {
	b.state = stateWaitingURL
	return c.Send("*Enter the website URL (e.g.: https://google.com):*", SendSettings)
}

func (b *Bot) handleDelete(c tele.Context) error {
	monitors, err := b.monitorService.GetAll()
	if err != nil {
		return c.Send("❌ *Failed to retrieve monitor list.*", SendSettings)
	}
	if len(monitors) == 0 {
		return c.Send("ℹ️ *No monitors available.*", SendSettings)
	}

	var sb strings.Builder
	sb.WriteString("*Select monitor ID to delete:*\n\n")
	for _, m := range monitors {
		sb.WriteString(fmt.Sprintf("*ID:* %d | `%s`\n", m.ID, m.URL))
	}

	b.state = stateWaitingDelID
	return c.Send(sb.String(), SendSettings)
}

func (b *Bot) handleStatus(c tele.Context) error {
	monitors, err := b.monitorService.GetAll()
	if err != nil {
		return c.Send("❌ *Failed to retrieve monitor list.*", SendSettings)
	}
	if len(monitors) == 0 {
		return c.Send("ℹ️ *No monitors available.*", SendSettings)
	}

	var sb strings.Builder
	sb.WriteString("*Select monitor ID:*\n\n")
	for _, m := range monitors {
		sb.WriteString(fmt.Sprintf("*ID:* %d | `%s`\n", m.ID, m.URL))
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
	case stateWaitingToggleID:
		return b.handleToggleID(c, text)
	default:
		return c.Send("❓ *Unknown command. Use /start to see available commands.*", SendSettings)
	}
}

func (b *Bot) handleAddURL(c tele.Context, url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return c.Send("❌ *URL must start with http:// or https://*\nTry again:", SendSettings)
	}
	b.state = stateIdle

	monitor, err := b.monitorService.Create(url, 60, 10)
	if err != nil {
		return c.Send("❌ *Failed to create monitor.*", SendSettings)
	}

	return c.Send(fmt.Sprintf(
		"✅ *Monitor added!*\n*ID:* %d\n*URL:* `%s`\n*Interval:* %ds",
		monitor.ID, monitor.URL, monitor.Interval,
	), SendSettings)
}

func (b *Bot) handleDeleteID(c tele.Context, text string) error {
	id, err := strconv.Atoi(text)
	if err != nil {
		return c.Send("❌ *Please enter a numeric monitor ID:*", SendSettings)
	}

	b.state = stateIdle

	if err := b.monitorService.Delete(id); err != nil {
		return c.Send("❌ *Monitor not found.*", SendSettings)
	}

	return c.Send(fmt.Sprintf("✅ *Monitor ID %d deleted.*", id), SendSettings)
}

func (b *Bot) handleStatusID(c tele.Context, text string) error {
	id, err := strconv.Atoi(text)
	if err != nil {
		return c.Send("❌ *Please enter a numeric monitor ID:*", SendSettings)
	}

	b.state = stateIdle

	check, err := b.checkService.GetLastByMonitorID(id)
	if err != nil {
		return c.Send("❌ *Monitor not found or no checks yet.*", SendSettings)
	}

	status := "✅ up"
	if !check.IsUp {
		status = "❌ down"
	}

	msg := fmt.Sprintf(
		"📊 *Last check:*\n*Status:* %s\n*Response time:* %d ms\n*Status code:* %d\n*Checked at:* %s",
		status,
		check.ResponseMs,
		check.StatusCode,
		check.CheckedAt.Format("02.01.2006 15:04:05"),
	)

	if check.Error != "" {
		msg += fmt.Sprintf("\n*Error:* %s", check.Error)
	}

	return c.Send(msg, SendSettings)
}

func (b *Bot) handleToggle(c tele.Context) error {
	monitors, err := b.monitorService.GetAll()
	if err != nil {
		return c.Send("❌ *Failed to retrieve monitor list.*", SendSettings)
	}
	if len(monitors) == 0 {
		return c.Send("ℹ️ *No monitors available.*", SendSettings)
	}

	var sb strings.Builder
	sb.WriteString("*Select monitor ID:*\n\n")
	for _, m := range monitors {
		status := "✅"
		if !m.IsActive {
			status = "⏸"
		}
		sb.WriteString(fmt.Sprintf("%s *ID:* %d | `%s`\n", status, m.ID, m.URL))
	}
	sb.WriteString("\n*Enter ID:*")

	b.state = stateWaitingToggleID
	return c.Send(sb.String(), SendSettings)
}

func (b *Bot) handleToggleID(c tele.Context, text string) error {
	id, err := strconv.Atoi(text)
	if err != nil {
		return c.Send("❌ *Please enter a numeric monitor ID:*", SendSettings)
	}

	b.state = stateIdle

	monitor, err := b.monitorService.GetByID(id)
	if err != nil {
		return c.Send("❌ *Monitor not found.*", SendSettings)
	}

	newStatus := !monitor.IsActive
	_, err = b.monitorService.Update(id, nil, nil, &newStatus)
	if err != nil {
		return c.Send("❌ *Failed to update monitor.*", SendSettings)
	}

	if newStatus {
		return c.Send(fmt.Sprintf("▶️ *Monitor ID %d enabled.*", id), SendSettings)
	}
	return c.Send(fmt.Sprintf("⏸ *Monitor ID %d disabled.*", id), SendSettings)
}
