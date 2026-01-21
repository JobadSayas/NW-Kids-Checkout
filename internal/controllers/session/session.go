package session

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

type Storer interface {
	RegisterType(i any)
	Get(c *fiber.Ctx) (*session.Session, error)
	Reset() error
	Delete(id string) error
}
