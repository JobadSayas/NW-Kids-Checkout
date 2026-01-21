package middleware

import (
	"fmt"
	"net/http"
	"net/url"

	"kids-checkin/internal/controllers/session"

	"github.com/gofiber/fiber/v2"
)

func AuthRequired(sessionStore session.Storer, allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, _ := sessionStore.Get(c)

		// Check if logged in
		if sess.Get("authenticated") != true {
			requestedURL := c.OriginalURL()
			if requestedURL == "" {
				requestedURL = c.Path()
			}
			return c.Redirect(fmt.Sprintf("/login?next=%s", url.QueryEscape(requestedURL)))
		}

		userRole, ok := sess.Get("role").(string)
		if !ok {
			return c.Status(http.StatusInternalServerError).SendString("Internal Server Error: Failed to fetch user role")
		}

		for _, role := range allowedRoles {
			if role == "" || userRole == role {
				return c.Next()
			}
		}

		return c.Status(http.StatusForbidden).SendString("Forbidden: Insufficient permissions")
	}
}
