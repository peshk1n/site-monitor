package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type IdentityStore interface {
	Identify(context.Context, int64, string) (models.User, error)
}
type Telegram struct {
	key    []byte
	users  IdentityStore
	maxAge time.Duration
	now    func() time.Time
}

func NewTelegram(token string, users IdentityStore, maxAge time.Duration) *Telegram {
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	var key []byte
	if token != "" {
		key = secret.Sum(nil)
	}
	return &Telegram{key: key, users: users, maxAge: maxAge, now: time.Now}
}
func (t *Telegram) Authenticate(ctx context.Context, header string) (models.User, error) {
	invalid := func() (models.User, error) { return models.User{}, service.ErrUnauthorized }
	if len(t.key) == 0 || !strings.HasPrefix(header, "tma ") || len(header) > 16384 {
		return invalid()
	}
	values, err := url.ParseQuery(strings.TrimPrefix(header, "tma "))
	if err != nil {
		return invalid()
	}
	lines := make([]string, 0, len(values))
	for key, value := range values {
		if len(value) != 1 {
			return invalid()
		}
		if key != "hash" {
			lines = append(lines, key+"="+value[0])
		}
	}
	sort.Strings(lines)
	received, err := hex.DecodeString(values.Get("hash"))
	if err != nil || len(received) != sha256.Size {
		return invalid()
	}
	mac := hmac.New(sha256.New, t.key)
	mac.Write([]byte(strings.Join(lines, "\n")))
	if !hmac.Equal(received, mac.Sum(nil)) {
		return invalid()
	}
	date, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return invalid()
	}
	now := t.now()
	issued := time.Unix(date, 0)
	if issued.After(now.Add(30*time.Second)) || now.Sub(issued) > t.maxAge {
		return invalid()
	}
	var user struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		IsBot     bool   `json:"is_bot"`
	}
	if json.Unmarshal([]byte(values.Get("user")), &user) != nil || user.ID <= 0 || user.IsBot {
		return invalid()
	}
	name := []rune(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if len(name) == 0 {
		return invalid()
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return t.users.Identify(ctx, user.ID, string(name))
}
