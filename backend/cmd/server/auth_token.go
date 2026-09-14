package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	accessTokenLifetime    = 2 * time.Hour
	maximumSessionLifetime = 5 * time.Hour
	tokenClockSkew         = time.Minute
)

type loginTokenData struct {
	UserID           int
	IssuedAt         time.Time
	SessionStartedAt time.Time
}

func newLoginTokenResponse(userID int, sessionStartedAt, now time.Time) map[string]any {
	sessionExpiresAt := sessionStartedAt.Add(maximumSessionLifetime)
	accessExpiresAt := now.Add(accessTokenLifetime)
	if accessExpiresAt.After(sessionExpiresAt) {
		accessExpiresAt = sessionExpiresAt
	}

	return map[string]any{
		"accessToken": fmt.Sprintf(
			"kol.%d.%d.%d",
			userID,
			now.Unix(),
			sessionStartedAt.Unix(),
		),
		"refreshToken":   fmt.Sprintf("kol.%d.refresh.%d", userID, sessionStartedAt.Unix()),
		"expires":        accessExpiresAt.Format("2006/01/02 15:04:05"),
		"sessionExpires": sessionExpiresAt.Format("2006/01/02 15:04:05"),
	}
}

func parseAccessToken(token string, now time.Time) (loginTokenData, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 && len(parts) != 4 || parts[0] != "kol" {
		return loginTokenData{}, false
	}
	userID, err := strconv.Atoi(parts[1])
	if err != nil || userID <= 0 {
		return loginTokenData{}, false
	}
	issuedUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return loginTokenData{}, false
	}
	issuedAt := time.Unix(issuedUnix, 0)
	sessionStartedAt := issuedAt
	if len(parts) == 4 {
		sessionUnix, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return loginTokenData{}, false
		}
		sessionStartedAt = time.Unix(sessionUnix, 0)
	}
	if issuedAt.After(now.Add(tokenClockSkew)) || sessionStartedAt.After(issuedAt) {
		return loginTokenData{}, false
	}
	if !now.Before(issuedAt.Add(accessTokenLifetime)) || !now.Before(sessionStartedAt.Add(maximumSessionLifetime)) {
		return loginTokenData{}, false
	}
	return loginTokenData{UserID: userID, IssuedAt: issuedAt, SessionStartedAt: sessionStartedAt}, true
}

func parseRefreshToken(token string, now time.Time) (loginTokenData, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 4 || parts[0] != "kol" || parts[2] != "refresh" {
		return loginTokenData{}, false
	}
	userID, err := strconv.Atoi(parts[1])
	if err != nil || userID <= 0 {
		return loginTokenData{}, false
	}
	sessionUnix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return loginTokenData{}, false
	}
	sessionStartedAt := time.Unix(sessionUnix, 0)
	if sessionStartedAt.After(now.Add(tokenClockSkew)) || !now.Before(sessionStartedAt.Add(maximumSessionLifetime)) {
		return loginTokenData{}, false
	}
	return loginTokenData{UserID: userID, SessionStartedAt: sessionStartedAt}, true
}
