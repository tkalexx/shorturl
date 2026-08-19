package handler

import (
	"net"
	"net/http"
)

// trustedSubnetMiddleware разрешает доступ только клиентам из доверенной подсети.
func trustedSubnetMiddleware(trustedSubnet string) func(http.Handler) http.Handler {
	forbidden := func(w http.ResponseWriter) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}

	if trustedSubnet == "" {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				forbidden(w)
			})
		}
	}

	_, network, err := net.ParseCIDR(trustedSubnet)
	if err != nil {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				forbidden(w)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := net.ParseIP(r.Header.Get("X-Real-IP"))
			if ip == nil || !network.Contains(ip) {
				forbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
