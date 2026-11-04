package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-bot-token"

var testNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

const testUserJSON = `{"id":42,"first_name":"Ada","last_name":"Lovelace","username":"ada","language_code":"en"}`

// signInitData builds a signed initData query string directly from the
// Telegram spec, independently of the production sign() helper.
func signInitData(t *testing.T, fields map[string]string, token string) string {
	t.Helper()

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + fields[k]
	}

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))

	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func validFields(authDate time.Time) map[string]string {
	return map[string]string{
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      testUserJSON,
		"auth_date": strconv.FormatInt(authDate.Unix(), 10),
		"signature": "ignored-by-hmac-check-but-part-of-data-check-string",
	}
}

func TestValidate_Valid(t *testing.T) {
	initData := signInitData(t, validFields(testNow.Add(-time.Hour)), testBotToken)

	got, err := Validate(initData, testBotToken, testNow)
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got.User.ID != 42 || got.User.FirstName != "Ada" || got.User.Username != "ada" {
		t.Errorf("unexpected user: %+v", got.User)
	}
	if !got.AuthDate.Equal(testNow.Add(-time.Hour)) {
		t.Errorf("AuthDate = %v, want %v", got.AuthDate, testNow.Add(-time.Hour))
	}
	if got.QueryID != "AAHdF6IQAAAAAN0XohDhrOrc" {
		t.Errorf("QueryID = %q", got.QueryID)
	}
}

func TestValidate_Invalid(t *testing.T) {
	fresh := testNow.Add(-time.Minute)
	valid := signInitData(t, validFields(fresh), testBotToken)

	tamper := func(key, val string) string {
		v, _ := url.ParseQuery(valid)
		v.Set(key, val)
		return v.Encode()
	}
	without := func(key string) string {
		v, _ := url.ParseQuery(valid)
		v.Del(key)
		return v.Encode()
	}
	signedWith := func(mutate func(map[string]string)) string {
		f := validFields(fresh)
		mutate(f)
		return signInitData(t, f, testBotToken)
	}

	tests := []struct {
		name     string
		initData string
		token    string
		want     error
	}{
		{"wrong bot token", valid, "999:other", ErrInvalidHash},
		{"no bot token configured", valid, "", ErrNoBotToken},
		{"tampered user", tamper("user", `{"id":1,"first_name":"Mallory"}`), testBotToken, ErrInvalidHash},
		{"tampered auth_date", tamper("auth_date", strconv.FormatInt(testNow.Unix(), 10)), testBotToken, ErrInvalidHash},
		{"extra unsigned field", valid + "&start_param=evil", testBotToken, ErrInvalidHash},
		{"missing hash", without("hash"), testBotToken, ErrMissingHash},
		{"non-hex hash", tamper("hash", "zz"), testBotToken, ErrInvalidHash},
		{"hash of wrong length", tamper("hash", "abcd"), testBotToken, ErrInvalidHash},
		{"duplicate key", valid + "&user=" + url.QueryEscape(testUserJSON), testBotToken, ErrMalformed},
		{"garbage", "%zz", testBotToken, ErrMalformed},
		{"empty", "", testBotToken, ErrMissingHash},
		{
			"expired (older than 24h)",
			signInitData(t, validFields(testNow.Add(-MaxAge-time.Second)), testBotToken),
			testBotToken, ErrExpired,
		},
		{
			"auth_date in the future",
			signInitData(t, validFields(testNow.Add(time.Hour)), testBotToken),
			testBotToken, ErrFromFuture,
		},
		{"missing auth_date", signedWith(func(f map[string]string) { delete(f, "auth_date") }), testBotToken, ErrMissingAuthDate},
		{"non-numeric auth_date", signedWith(func(f map[string]string) { f["auth_date"] = "yesterday" }), testBotToken, ErrMissingAuthDate},
		{"missing user", signedWith(func(f map[string]string) { delete(f, "user") }), testBotToken, ErrMissingUser},
		{"user is not JSON", signedWith(func(f map[string]string) { f["user"] = "nope" }), testBotToken, ErrMissingUser},
		{"user without id", signedWith(func(f map[string]string) { f["user"] = `{"first_name":"X"}` }), testBotToken, ErrMissingUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.initData, tt.token, testNow)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidate_ExactlyAtMaxAgeIsAccepted(t *testing.T) {
	initData := signInitData(t, validFields(testNow.Add(-MaxAge)), testBotToken)
	if _, err := Validate(initData, testBotToken, testNow); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
