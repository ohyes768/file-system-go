# Final Fix Report

## 2026-07-17 — M1/M2 must-fix (feat/nas-file-ui)

### M1: /static/index.html bypasses UI password
- **Fix:** Replaced unfiltered `FileServer` on `/static/` with `uiStaticHandler` that only serves `.js` and `.css` files.
- **Result:** `/static/index.html` and `/static/login.html` return 404; HTML is only served via `uiIndexHandler` (cookie-gated) and `uiLoginPageHandler`.
- **Files:** `ui_auth.go`, `main.go`, `ui_auth_test.go`

### M2: Subdir delete can delete wrong root entries
- **Fix:** Hide Delete button when `currentPath` is non-empty; `deleteEntry` no-ops with alert as defense-in-depth.
- **UI note:** Added hint "删除仅支持根目录" in `index.html`.
- **Files:** `web/app.js`, `web/index.html`

### Verification
- `go test ./...` — pass
- `go build` — pass
