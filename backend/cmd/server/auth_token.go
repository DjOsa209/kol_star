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
	SessionID        string
}

func newLoginTokenResponse(userID int, sessionStartedAt, now time.Time, sessionID string) map[string]any {
	sessionExpiresAt := sessionStartedAt.Add(maximumSessionLifetime)
	accessExpiresAt := now.Add(accessTokenLifetime)
	if accessExpiresAt.After(sessionExpiresAt) {
		accessExpiresAt = sessionExpiresAt
	}

	return map[string]any{
		"accessToken": fmt.Sprintf(
			"kol.%d.%d.%d.%s",
			userID,
			now.Unix(),
			sessionStartedAt.Unix(),
			sessionID,
		),
		"refreshToken":   fmt.Sprintf("kol.%d.refresh.%d.%s", userID, sessionStartedAt.Unix(), sessionID),
		"expires":        accessExpiresAt.Format("2006/01/02 15:04:05"),
		"sessionExpires": sessionExpiresAt.Format("2006/01/02 15:04:05"),
	}
}

func parseAccessToken(token string, now time.Time) (loginTokenData, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 5 || parts[0] != "kol" || !validSessionID(parts[4]) {
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
	sessionUnix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return loginTokenData{}, false
	}
	sessionStartedAt := time.Unix(sessionUnix, 0)
	if issuedAt.After(now.Add(tokenClockSkew)) || sessionStartedAt.After(issuedAt) {
		return loginTokenData{}, false
	}
	if !now.Before(issuedAt.Add(accessTokenLifetime)) || !now.Before(sessionStartedAt.Add(maximumSessionLifetime)) {
		return loginTokenData{}, false
	}
	return loginTokenData{UserID: userID, IssuedAt: issuedAt, SessionStartedAt: sessionStartedAt, SessionID: parts[4]}, true
}

func parseRefreshToken(token string, now time.Time) (loginTokenData, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 5 || parts[0] != "kol" || parts[2] != "refresh" || !validSessionID(parts[4]) {
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
	return loginTokenData{UserID: userID, SessionStartedAt: sessionStartedAt, SessionID: parts[4]}, true
}

func validSessionID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
