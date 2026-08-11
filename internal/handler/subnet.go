package handler

import (
	"net"
	"net/http"
)

// trustedSubnetMiddleware разрешает доступ только клиентам из доверенной подсети.
func trustedSubnetMiddleware(trustedSubnet string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trustedSubnet == "" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			_, network, err := net.ParseCIDR(trustedSubnet)
			if err != nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			ip := net.ParseIP(r.Header.Get("X-Real-IP"))
			if ip == nil || !network.Contains(ip) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
