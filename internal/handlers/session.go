package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey int

const sessionKey ctxKey = 0

const sessionCookie = "tp_session"

// session ensures every visitor has a stable session cookie (used to key the
// cart) and puts its value on the request context.
func (h *Handlers) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sid string
		if c, err := r.Cookie(sessionCookie); err == nil && len(c.Value) >= 16 {
			sid = c.Value
		} else {
			buf := make([]byte, 16)
			rand.Read(buf)
			sid = hex.EncodeToString(buf)
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookie,
				Value:    sid,
				Path:     "/",
				MaxAge:   180 * 24 * 60 * 60,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, sid)))
	})
}

func sessionID(r *http.Request) string {
	sid, _ := r.Context().Value(sessionKey).(string)
	return sid
}
