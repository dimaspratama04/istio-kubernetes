# Istio Service Mesh on Kubernetes: Go Fiber Microservices

This project demonstrates a multi-service architecture using **Istio Service Mesh** on Kubernetes. It consists of two Go Fiber services: a `product-service` (entry point) and a `payment-service` (internal backend).

## Prerequisites

- **Kubernetes Cluster** (Minikube, Docker Desktop, or GKE/EKS)
- **kubectl** CLI
- **Docker** CLI
- **Go 1.21+** (for local development)

---

## 1. Istio Installation

Before deploying the services, you must install Istio and its Custom Resource Definitions (CRDs) on your cluster.

```bash
# 1. Download Istio (Mac/Linux)
curl -L https://istio.io/downloadIstio | sh -

# 2. Move to the Istio directory
cd istio-*

# 3. Add istioctl to your PATH (Temporary for this session)
export PATH=$PWD/bin:$PATH

# 4. Install Istio with the 'demo' profile
# This installs the Control Plane (istiod), Ingress/Egress Gateways, and all CRDs.
istioctl install --set profile=demo -y

# 5. Return to the project root
cd ..
```

---

## 2. Deployment Steps

### 1. Setup Namespace with Istio Injection
Create a dedicated namespace and enable automatic Istio sidecar injection.

```bash
kubectl create namespace rnd-services
kubectl label namespace rnd-services istio-injection=enabled
```

### 2. Build Docker Images
Build the services locally. If using Minikube, remember to point your shell to Minikube's Docker daemon first: `eval $(minikube docker-env)`.

```bash
# Build Payment Service
docker build -t rnd-registry/payment-service:latest ./payment-service

# Build Product Service
docker build -t rnd-registry/product-service:latest ./product-service
```

### 3. Deploy to Kubernetes
Apply the deployments, services, and Istio gateway rules.

```bash
kubectl apply -f k8s/ -n rnd-services
```

### 4. Verify Deployment
Wait for the pods to be ready. You should see `2/2` containers in each pod (Application + Envoy Sidecar).

```bash
kubectl get pods -n rnd-services
```

---

## Testing the Architecture

### 1. Configure Host Resolution
The Istio VirtualService is configured for the host `api.rnd-istio.local`. 

**Option A: Add to `/etc/hosts`**
Find your Ingress IP (e.g., `minikube ip`) and add it:
`127.0.0.1 api.rnd-istio.local` (Replace 127.0.0.1 with your cluster IP)

**Option B: Use Curl Header (Recommended for quick testing)**
```bash
# Get Ingress Host and Port
export INGRESS_HOST=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}')

# If using Minikube and LoadBalancer is not available:
# export INGRESS_HOST=$(minikube ip)
# export INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}')

curl -H "Host: api.rnd-istio.local" http://$INGRESS_HOST:$INGRESS_PORT/api/products/123
```

### 2. Expected Response
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

---

## Features Demonstrated
- **Sidecar Injection**: Automatic Envoy proxy management.
- **Service Discovery**: `product-service` calls `payment-service` via internal K8s DNS.
- **Header Propagation**: `product-service` extracts and forwards `x-request-id` and B3 headers for distributed tracing.
- **Traffic Management**: Istio Gateway and VirtualService handling external ingress.
- **Zero Trust**: Traffic is automatically secured via mTLS between sidecars.
