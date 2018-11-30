package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/sirupsen/logrus"

	"github.com/supergiant/control/pkg/sgerrors"
)

type TokenValidater interface {
	Validate(string) (jwt.MapClaims, error)
}

type Middleware struct {
	TokenService TokenValidater
}

func (m *Middleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// In case of websocket protocol look at the header Sec-Websocket-Protocol
		if authHeader == "" {
			logrus.Debugf("Websocket header Sec-Websocket-Protocol %s",
				r.Header.Get("Sec-Websocket-Protocol"))
			authHeader = fmt.Sprintf("Bearer: %s",
				r.Header.Get("Sec-Websocket-Protocol"))
		}

		if authHeader == "" {
			http.Error(w, sgerrors.ErrInvalidCredentials.Error(), http.StatusForbidden)
			return
		}

		if ts := strings.Split(authHeader, " "); len(ts) <= 1 {
			http.Error(w, sgerrors.ErrInvalidCredentials.Error(), http.StatusForbidden)
			return
		} else {
			tokenString := ts[1]
			claims, err := m.TokenService.Validate(tokenString)

			if err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}

			// TODO(stgleb): Do something with claims
			userId, ok := claims["user_id"].(string)
			if !ok {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}

			if len(userId) == 0 {
				http.Error(w, "unknown user", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}
	})
}

func ContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
