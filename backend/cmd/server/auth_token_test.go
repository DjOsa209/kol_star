package main

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccessTokenHonorsAccessAndAbsoluteSessionExpiry(t *testing.T) {
	sessionStartedAt := time.Unix(1_800_000_000, 0)
	sessionID := strings.Repeat("a", 64)

	fresh := fmt.Sprintf("kol.42.%d.%d.%s", sessionStartedAt.Add(90*time.Minute).Unix(), sessionStartedAt.Unix(), sessionID)
	if data, ok := parseAccessToken(fresh, sessionStartedAt.Add(2*time.Hour)); !ok || data.UserID != 42 {
		t.Fatalf("fresh access token rejected: data=%+v ok=%v", data, ok)
	}

	accessExpired := fmt.Sprintf("kol.42.%d.%d.%s", sessionStartedAt.Unix(), sessionStartedAt.Unix(), sessionID)
	if _, ok := parseAccessToken(accessExpired, sessionStartedAt.Add(accessTokenLifetime)); ok {
		t.Fatal("access token was accepted at its two-hour expiry")
	}

	nearSessionEnd := fmt.Sprintf("kol.42.%d.%d.%s", sessionStartedAt.Add(4*time.Hour+59*time.Minute).Unix(), sessionStartedAt.Unix(), sessionID)
	if _, ok := parseAccessToken(nearSessionEnd, sessionStartedAt.Add(maximumSessionLifetime)); ok {
		t.Fatal("access token was accepted at the five-hour absolute session expiry")
	}
}

func TestRefreshTokenDoesNotSlideFiveHourSession(t *testing.T) {
	sessionStartedAt := time.Unix(1_800_000_000, 0)
	sessionID := strings.Repeat("b", 64)
	token := fmt.Sprintf("kol.7.refresh.%d.%s", sessionStartedAt.Unix(), sessionID)

	data, ok := parseRefreshToken(token, sessionStartedAt.Add(4*time.Hour+59*time.Minute))
	if !ok || data.SessionStartedAt.Unix() != sessionStartedAt.Unix() {
		t.Fatalf("valid refresh token rejected: data=%+v ok=%v", data, ok)
	}
	refreshed := newLoginTokenResponse(data.UserID, data.SessionStartedAt, sessionStartedAt.Add(4*time.Hour+59*time.Minute), data.SessionID)
	if got := refreshed["refreshToken"]; got != token {
		t.Fatalf("refresh token slid the session start: got %v want %v", got, token)
	}
	wantExpiry := sessionStartedAt.Add(maximumSessionLifetime).Format("2006/01/02 15:04:05")
	if got := refreshed["expires"]; got != wantExpiry {
		t.Fatalf("access expiry exceeded session cap: got %v want %v", got, wantExpiry)
	}

	if _, ok := parseRefreshToken(token, sessionStartedAt.Add(maximumSessionLifetime)); ok {
		t.Fatal("refresh token was accepted at the five-hour absolute session expiry")
	}
}

func TestLegacyUnsignedTokensAreRejected(t *testing.T) {
	now := time.Now()
	if _, ok := parseAccessToken(fmt.Sprintf("kol.7.%d.%d", now.Unix(), now.Unix()), now); ok {
		t.Fatal("unsigned legacy access token accepted")
	}
	if _, ok := parseRefreshToken(fmt.Sprintf("kol.7.refresh.%d", now.Unix()), now); ok {
		t.Fatal("unsigned legacy refresh token accepted")
	}
}

func TestCurrentUserIDRejectsExpiredAccessToken(t *testing.T) {
	issuedAt := time.Now().Add(-accessTokenLifetime - time.Minute)
	request := httptest.NewRequest("GET", "/api/mine", nil)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer kol.9.%d", issuedAt.Unix()))

	if userID, ok := (&app{}).currentUserID(request); ok || userID != 0 {
		t.Fatalf("expired access token accepted: userID=%d ok=%v", userID, ok)
	}
}
