package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"audio-server/web"
)

const uiSessionCookie = "ui_session"

type uiLoginRequest struct {
	Password string `json:"password"`
}

func issueUISession(now time.Time) string {
	expiry := now.Add(time.Duration(config.UI.SessionDays) * 24 * time.Hour)
	return signUISession(expiry)
}

func signUISession(expiry time.Time) string {
	msg := strconv.FormatInt(expiry.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(config.UI.Password))
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString([]byte(msg + "|" + hex.EncodeToString(mac.Sum(nil))))
}

func validUISession(token string, now time.Time) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return false
	}
	expiryUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	expiry := time.Unix(expiryUnix, 0)
	if !now.Before(expiry) {
		return false
	}
	msg := parts[0]
	mac := hmac.New(sha256.New, []byte(config.UI.Password))
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}

func uiSessionFromRequest(r *http.Request) (string, bool) {
	c, err := r.Cookie(uiSessionCookie)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, validUISession(c.Value, time.Now())
}

func setUISessionCookie(w http.ResponseWriter, now time.Time) {
	expiry := now.Add(time.Duration(config.UI.SessionDays) * 24 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     uiSessionCookie,
		Value:    signUISession(expiry),
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearUISessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     uiSessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func uiLoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req uiLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(Response{Success: false, Error: "invalid credentials"})
		return
	}
	if req.Password != config.UI.Password {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(Response{Success: false, Error: "invalid credentials"})
		return
	}
	setUISessionCookie(w, time.Now())
	json.NewEncoder(w).Encode(Response{Success: true})
}

func uiLogoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	clearUISessionCookie(w)
	json.NewEncoder(w).Encode(Response{Success: true})
}

func serveWebFile(w http.ResponseWriter, r *http.Request, name string) {
	data, err := web.Files.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	w.Write(data)
}

func uiIndexHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := uiSessionFromRequest(r); !ok {
		http.Redirect(w, r, "/login.html", http.StatusFound)
		return
	}
	serveWebFile(w, r, "index.html")
}

func uiLoginPageHandler(w http.ResponseWriter, r *http.Request) {
	serveWebFile(w, r, "login.html")
}

func requireUIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ui/logout" {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := uiSessionFromRequest(r); !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{Success: false, Error: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
