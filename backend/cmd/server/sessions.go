package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"
)

const onlineActivityWindow = 15 * time.Minute

func newSessionID() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return hex.EncodeToString(secret), nil
}

func sessionHash(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])
}

func (a *app) createSession(r *http.Request, userID int, username, sessionID string, startedAt time.Time) error {
	ip, address, systemName, browser := requestSecurityInfo(r)
	_, err := a.DB().ExecContext(r.Context(),
		`insert into sys_sessions
		  (session_hash, user_id, username, ip, address, `+"`system`"+`, browser,
		   started_at_unix, last_seen_unix, expires_at_unix)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionHash(sessionID), userID, username, ip, address, systemName, browser,
		startedAt.Unix(), startedAt.Unix(), startedAt.Add(maximumSessionLifetime).Unix(),
	)
	return err
}

// Session validation fails closed if the database cannot confirm this token.
func (a *app) activeSession(ctx context.Context, token loginTokenData, now time.Time) (bool, error) {
	var lastSeen int64
	err := a.DB().QueryRowContext(ctx,
		`select s.last_seen_unix from sys_sessions s
		  join sys_users u on u.id = s.user_id and u.status = 1
		 where s.session_hash = ? and s.user_id = ? and s.started_at_unix = ?
		   and s.revoked_at_unix is null and s.expires_at_unix > ? limit 1`,
		sessionHash(token.SessionID), token.UserID, token.SessionStartedAt.Unix(), now.Unix(),
	).Scan(&lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if now.Unix()-lastSeen >= 60 {
		if _, err := a.DB().ExecContext(ctx,
			`update sys_sessions set last_seen_unix = ?
			 where session_hash = ? and revoked_at_unix is null and expires_at_unix > ?`,
			now.Unix(), sessionHash(token.SessionID), now.Unix(),
		); err != nil {
			log.Printf("update session activity failed: %v", err)
		}
	}
	return true, nil
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	token, ok := accessTokenFromRequest(r)
	if !ok {
		// An expired access token may still belong to a live five-hour session.
		// The refresh token lets the user revoke that session explicitly.
		token, ok = parseRefreshToken(stringField(readBody(r), "refreshToken"), now)
		if !ok {
			writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
			return
		}
	}
	active, err := a.activeSession(r.Context(), token, now)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if !active {
		writeError(w, http.StatusUnauthorized, 401, "登录会话已失效")
		return
	}
	if _, err := a.DB().ExecContext(r.Context(),
		`update sys_sessions set revoked_at_unix = ? where session_hash = ? and revoked_at_unix is null`,
		now.Unix(), sessionHash(token.SessionID),
	); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"loggedOut": true})
}

func accessTokenFromRequest(r *http.Request) (loginTokenData, bool) {
	token := r.Header.Get("Authorization")
	if len(token) < len("Bearer ") || token[:len("Bearer ")] != "Bearer " {
		return loginTokenData{}, false
	}
	return parseAccessToken(token[len("Bearer "):], time.Now())
}
