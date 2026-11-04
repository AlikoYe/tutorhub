// Package auth validates Telegram Mini App initData and exposes the
// authenticated user to handlers via the request context.
//
// Spec: https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MaxAge is how old auth_date may be before initData is rejected.
const MaxAge = 24 * time.Hour

// maxClockSkew tolerates small clock differences between Telegram and us
// for auth_date values slightly in the future.
const maxClockSkew = time.Minute

var (
	ErrMalformed       = errors.New("init data is malformed")
	ErrMissingHash     = errors.New("init data has no hash")
	ErrInvalidHash     = errors.New("init data signature is invalid")
	ErrMissingAuthDate = errors.New("init data has no valid auth_date")
	ErrExpired         = errors.New("init data has expired")
	ErrFromFuture      = errors.New("init data auth_date is in the future")
	ErrMissingUser     = errors.New("init data has no valid user")
	ErrNoBotToken      = errors.New("bot token is not configured")
)

// User is the Telegram user embedded in initData.
type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
	IsPremium    bool   `json:"is_premium,omitempty"`
	PhotoURL     string `json:"photo_url,omitempty"`

	// Role comes from the users table; set by Middleware, never by initData.
	Role string `json:"-"`
}

// DisplayName is "First Last" without extra spaces.
func (u User) DisplayName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

// InitData is the validated, parsed initData.
type InitData struct {
	User       User
	AuthDate   time.Time
	QueryID    string
	StartParam string
}

// Validate checks the initData signature against botToken and its freshness
// against now, then parses it. The error is one of the Err* values above.
func Validate(initData, botToken string, now time.Time) (InitData, error) {
	if botToken == "" {
		return InitData{}, ErrNoBotToken
	}

	values, err := url.ParseQuery(initData)
	if err != nil {
		return InitData{}, ErrMalformed
	}
	// Each key must appear exactly once; duplicates would make the
	// data-check-string ambiguous.
	for _, v := range values {
		if len(v) != 1 {
			return InitData{}, ErrMalformed
		}
	}

	rawHash := values.Get("hash")
	if rawHash == "" {
		return InitData{}, ErrMissingHash
	}
	gotHash, err := hex.DecodeString(rawHash)
	if err != nil {
		return InitData{}, ErrInvalidHash
	}
	if !hmac.Equal(gotHash, sign(values, botToken)) {
		return InitData{}, ErrInvalidHash
	}

	// Only trust the contents once the signature is verified.
	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil || authUnix <= 0 {
		return InitData{}, ErrMissingAuthDate
	}
	authDate := time.Unix(authUnix, 0)
	if now.Sub(authDate) > MaxAge {
		return InitData{}, ErrExpired
	}
	if authDate.Sub(now) > maxClockSkew {
		return InitData{}, ErrFromFuture
	}

	var user User
	if raw := values.Get("user"); raw == "" || json.Unmarshal([]byte(raw), &user) != nil || user.ID == 0 {
		return InitData{}, ErrMissingUser
	}

	return InitData{
		User:       user,
		AuthDate:   authDate,
		QueryID:    values.Get("query_id"),
		StartParam: values.Get("start_param"),
	}, nil
}

// sign returns HMAC_SHA256(key=secret, msg=data-check-string) where
// secret = HMAC_SHA256(key="WebAppData", msg=botToken).
func sign(values url.Values, botToken string) []byte {
	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	return hmacSHA256(secret, []byte(dataCheckString(values)))
}

// dataCheckString joins every field except "hash" as key=value, sorted by
// key, separated by "\n".
func dataCheckString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s=%s", k, values.Get(k))
	}
	return b.String()
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
