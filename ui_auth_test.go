package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUISession_RoundTrip(t *testing.T) {
	config.UI.Password = "secret"
	config.UI.SessionDays = 7
	token := issueUISession(time.Now())
	if !validUISession(token, time.Now()) {
		t.Fatal("expected valid")
	}
	if validUISession(token, time.Now().Add(8*24*time.Hour)) {
		t.Fatal("expected expired")
	}
	if validUISession("tampered", time.Now()) {
		t.Fatal("expected invalid")
	}
}

func TestLoginHandler_SetsCookie(t *testing.T) {
	config.UI.Password = "secret"
	config.UI.SessionDays = 7
	req := httptest.NewRequest(http.MethodPost, "/api/ui/login", strings.NewReader(`{"password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	uiLoginHandler(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "ui_session" {
		t.Fatalf("missing cookie: %+v", cookies)
	}
}

func TestUIIndexHandler_RedirectsWithoutCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	uiIndexHandler(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("code=%d want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login.html" {
		t.Fatalf("location=%q want /login.html", loc)
	}
}

func TestUILoginPageHandler_ServesHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/login.html", nil)
	rr := httptest.NewRecorder()
	uiLoginPageHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "文件管理") {
		t.Fatal("expected login page content")
	}
}
