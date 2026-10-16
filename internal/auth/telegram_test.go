package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

type identities struct{ calls int }

func (i *identities) Identify(ctx context.Context, id int64, name string) (models.User, error) {
	i.calls++
	return models.User{ID: 7, TelegramID: id, DisplayName: name}, nil
}
func signed(date int64) string {
	user := `{"id":1099511627776,"first_name":"Test"}`
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte("test-token"))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte("auth_date=" + strconv.FormatInt(date, 10) + "\nuser=" + user))
	return url.Values{"auth_date": {strconv.FormatInt(date, 10)}, "user": {user}, "hash": {hex.EncodeToString(mac.Sum(nil))}}.Encode()
}
func TestTelegramAuthentication(t *testing.T) {
	now := time.Now()
	users := &identities{}
	a := NewTelegram("test-token", users, 15*time.Minute)
	a.now = func() time.Time { return now }
	user, err := a.Authenticate(context.Background(), "tma "+signed(now.Unix()))
	if err != nil || user.TelegramID != 1099511627776 {
		t.Fatalf("valid identity rejected: %v", err)
	}
	tampered, _ := url.ParseQuery(signed(now.Unix()))
	tampered.Set("user", `{"id":1,"first_name":"Test"}`)
	for _, header := range []string{"", "Bearer 7", "tma " + tampered.Encode(), "tma " + signed(now.Add(-time.Hour).Unix()), "tma " + signed(now.Add(time.Minute).Unix()), "tma " + signed(now.Unix()) + "&auth_date=1"} {
		if _, err = a.Authenticate(context.Background(), header); err == nil {
			t.Fatal("invalid authorization accepted")
		}
	}
	if users.calls != 1 {
		t.Fatal("unverified identity reached repository")
	}
}
