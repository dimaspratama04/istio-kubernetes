# Istio Service Mesh on Kubernetes: Multi-Service Architecture and R&D Guide

This Research and Development (R&D) guide explores the implementation of **Istio Service Mesh** within a Kubernetes cluster to manage communication between multiple backend microservices. 

To demonstrate Istio's service-to-service capabilities, this guide uses a sample architecture consisting of two **Golang (Go Fiber)** backend services: a `product-service` and a `payment-service` communicating within the same cluster.

---

## 1. Architecture & Workflow

In this architecture, the `product-service` acts as the entry point for external API requests. To fulfill a request, it must internally communicate with the `payment-service`.

Istio manages this entire flow by injecting Envoy proxy sidecars into both application pods.

```text
                                  Control Plane (Istiod)
                                            |
+-------------------------------------------|-----------------------------------+
| Data Plane (Kubernetes Cluster)           v                                   |
|                                                                               |
|                     [ Pod: Product Service ]        [ Pod: Payment Service ]  |
|                     +--------------------+          +--------------------+    |
|  External Request   |    Envoy Proxy     |          |    Envoy Proxy     |    |
| ------------------->|    (Sidecar)       |--------->|    (Sidecar)       |    |
| (via Ingress GW)    +--------------------+   mTLS   +--------------------+    |
|                               |                         |                     |
|                               v                         v                     |
|                     +--------------------+          +--------------------+    |
|                     |    Go Fiber API    |          |    Go Fiber API    |    |
|                     | (product-service)  |          | (payment-service)  |    |
|                     +--------------------+          +--------------------+    |
+-------------------------------------------------------------------------------+
```

### Key Istio Concepts Demonstrated:
1.  **Service-to-Service Communication**: The `product-service` calls the `payment-service` using standard Kubernetes internal DNS (`http://payment-service:8080`). The Envoy proxy intercepts this outbound call, routes it, and load-balances it.
2.  **Automatic mTLS**: Istio automatically encrypts the traffic between the `product` and `payment` pods. You do not need to configure HTTPS or certificates in your Golang code.
3.  **Distributed Tracing (Header Propagation)**: To trace a request as it hops from the Ingress Gateway $\rightarrow$ Product Service $\rightarrow$ Payment Service, the application code must propagate specific HTTP headers (like `x-request-id`).

---

## 2. Sample Project: Product and Payment Services

### Project Structure
```text
rnd-istio-fiber/
├── product-service/
│   ├── main.go
│   └── Dockerfile
├── payment-service/
│   ├── main.go
│   └── Dockerfile
└── k8s/
    ├── istio-gateway.yaml
    ├── product-deploy.yaml
    └── payment-deploy.yaml
```

---

### A. Application Code (Golang + Go Fiber)

#### 1. Payment Service (`payment-service/main.go`)
This service acts as an internal backend API. It does not need to propagate tracing headers further because it is the end of the chain.

```go
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
```

#### 2. Product Service (`product-service/main.go`)
This service handles the initial request and makes an HTTP call to the `payment-service`. **Crucially, it extracts and forwards Istio's tracing headers** so the mesh can map the entire request lifecycle.

```go
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
```

#### 3. Dockerfile (Reusable for both services)
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
```

---

### B. Kubernetes & Istio Manifests (`k8s/`)

#### 1. Payment Service Deployment (`k8s/payment-deploy.yaml`)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-service
  labels:
    app: payment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: payment
  template:
    metadata:
      labels:
        app: payment
    spec:
      containers:
      - name: payment
        image: rnd-registry/payment-service:latest
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: payment-service
spec:
  ports:
  - port: 8080
    targetPort: 8080
    name: http
  selector:
    app: payment
```

#### 2. Product Service Deployment (`k8s/product-deploy.yaml`)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: product-service
  labels:
    app: product
spec:
  replicas: 1
  selector:
    matchLabels:
      app: product
  template:
    metadata:
      labels:
        app: product
    spec:
      containers:
      - name: product
        image: rnd-registry/product-service:latest
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: product-service
spec:
  ports:
  - port: 8080
    targetPort: 8080
    name: http
  selector:
    app: product
```

#### 3. Istio Ingress Gateway (`k8s/istio-gateway.yaml`)
This routes all external traffic exclusively to the `product-service`. The `payment-service` remains completely isolated from the outside world, reachable only from within the mesh.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: api-gateway
spec:
  selector:
    istio: ingressgateway # Uses default Istio ingress gateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - "api.rnd-istio.local"
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: product-routing
spec:
  hosts:
  - "api.rnd-istio.local"
  gateways:
  - api-gateway
  http:
  - match:
    - uri:
        prefix: /api/products
    route:
    - destination:
        host: product-service
        port:
          number: 8080
```

---

## 3. Execution and Testing Workflow

1.  **Enable Istio Proxy Injection**:
    ```bash
    kubectl create namespace rnd-services
    kubectl label namespace rnd-services istio-injection=enabled
    ```

2.  **Deploy Both Services**:
    ```bash
    kubectl apply -f k8s/payment-deploy.yaml -n rnd-services
    kubectl apply -f k8s/product-deploy.yaml -n rnd-services
    ```

3.  **Apply the Ingress Routing Rules**:
    ```bash
    kubectl apply -f k8s/istio-gateway.yaml -n rnd-services
    ```

4.  **Test the Integration**:
    Send an HTTP request to the product endpoint. You will receive a combined payload demonstrating that the `product-service` successfully reached the `payment-service` through the Envoy mesh.
    ```bash
    curl http://api.rnd-istio.local/api/products/123
    ```

    *Expected Output:*
    ```json
    {
      "name": "Premium Cloud Subscription",
      "payment_dependency": {
        "code": 200,
        "service": "payment-service",
        "status": "Payment Gateway is Online"
      },
      "price": 99.99,
      "product_id": "123"
    }
    ```

## 4. Conclusion
By decoupling the architecture into two distinct backend services within the same mesh, the operations team can now apply fine-grained Istio policies. For example, you can implement an `AuthorizationPolicy` that strictly allows the `product-service` identity to invoke the `payment-service`, establishing a zero-trust network environment entirely abstracted from the Go application code.
