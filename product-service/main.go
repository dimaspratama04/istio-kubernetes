package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	app := fiber.New()
	app.Use(logger.New())

	app.Get("/api/products/:id", func(c *fiber.Ctx) error {
		// 1. Extract Istio tracing headers from the incoming request
		traceHeaders := []string{
			"x-request-id",
			"x-b3-traceid",
			"x-b3-spanid",
			"x-b3-sampled",
			"x-b3-flags",
			"x-ot-span-context",
		}

		// 2. Prepare the request to the internal payment service
		paymentURL := "http://payment-service:8080/api/payments/status"
		req, err := http.NewRequest("GET", paymentURL, nil)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create request"})
		}

		// 3. Propagate the headers to the outgoing request
		for _, header := range traceHeaders {
			if val := c.Get(header); val != "" {
				req.Header.Set(header, val)
			}
		}

		// 4. Execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Payment service is unreachable"})
		}
		defer resp.Body.Close()

		// 5. Parse response and return aggregated payload
		body, _ := io.ReadAll(resp.Body)
		var paymentData map[string]interface{}
		json.Unmarshal(body, &paymentData)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"product_id": c.Params("id"),
			"name":       "Premium Cloud Subscription",
			"price":      99.99,
			"payment_dependency": paymentData,
		})
	})

	log.Fatal(app.Listen(":8080"))
}
