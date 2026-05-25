package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	app := fiber.New()
	app.Use(logger.New())

	app.Get("/api/payments/status", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"service": "payment-service",
			"status":  "Payment Gateway is Online",
			"code":    200,
		})
	})

	log.Fatal(app.Listen(":8080"))
}
