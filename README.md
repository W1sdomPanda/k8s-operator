# Game Event Scaler Operator

Kubernetes operator for automatic scaling of microservices based on game events. The operator monitors external event APIs and automatically scales corresponding Deployments in the Kubernetes cluster.

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Quick Start (Local with Kind)](#quick-start-local-with-kind)
    - [Cluster Setup](#cluster-setup)
    - [Build and Load Images](#build-and-load-images)
    - [Deploy Test API Server](#deploy-test-api-server)
    - [Update Test Server (After Code Changes)](#update-test-server-after-code-changes)
    - [Create Test Deployments](#create-test-deployments)
    - [Configure and Apply Custom Resource](#configure-and-apply-custom-resource)
    - [Operator Logs & Monitoring](#operator-logs--monitoring)
4. [Cleanup](#cleanup)
5. [Production Deployment](#production-deployment)
6. [Project Structure](#project-structure)
7. [Troubleshooting](#troubleshooting)
8. [License](#license)

---

## Overview

Game Event Scaler Operator:
- Periodically polls external APIs to retrieve game event information
- Automatically scales microservices based on active events
- Supports various event types (PvP, Raid Boss, etc.)
- Provides detailed information about scaling status

---

## Prerequisites

- Go 1.24+
- Docker
- kubectl
- Kind (Kubernetes in Docker)

---

## Quick Start (Local with Kind)

### 1. Cluster Setup

```bash
# Install Kind (if needed)
brew install kind  # macOS
# or
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.29.0/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind

# Create Kind cluster
kind create cluster --name k8s-operator-test

# Check cluster status
kubectl cluster-info --context kind-k8s-operator-test
```

### 2. Build and Load Images

```bash
# Install CRDs
make install

# Build operator image
make docker-build IMG=controller:latest

# Load operator image into Kind
kind load docker-image controller:latest --name k8s-operator-test
```

### 3. Deploy Test API Server

```bash
# Build test server image
cd test-server
docker build -t game-event-api:latest .
cd ..

# Load test server image into Kind
kind load docker-image game-event-api:latest --name k8s-operator-test

# Deploy test server
kubectl apply -f test-server/k8s-deployment.yaml

# Verify test server
kubectl get pods -l app=game-event-api
kubectl get svc game-event-api
```

### 3.1. Update Test Server (After Code Changes)

When you modify the test server code, you need to rebuild and redeploy:

```bash
# Rebuild test server image
cd test-server
docker build -t game-event-api:latest .
cd ..

# Load updated image into Kind
kind load docker-image game-event-api:latest --name k8s-operator-test

# Restart test server deployment
kubectl rollout restart deployment/game-event-api

# Verify the update
kubectl get pods -l app=game-event-api
kubectl logs -l app=game-event-api --tail=10
```

### 4. Create Test Deployments

> The operator will only scale Deployments that exist and match the `targetMicroservice` names in your Custom Resource.

**Option 1: CLI**
```bash
kubectl create deployment pvp-battle-service --image=nginx:alpine --replicas=3
kubectl create deployment raid-instance-manager --image=nginx:alpine --replicas=2
```

**Option 2: YAML**
```bash
kubectl apply -f test-deployments.yaml
```
(see `test-deployments.yaml` in the repo)

### 5. Configure and Apply Custom Resource

Edit `config/samples/game_v1_gameeventscalerule.yaml` if needed:
```yaml
spec:
  eventEndpointURL: "http://game-event-api.default.svc.cluster.local/api/events"
  pollingInterval: "30s"
  rules:
    - eventType: "MassPvPEvent"
      targetMicroservice: "pvp-battle-service"
      defaultReplicas: 3
      desiredReplicas: 20
      preScaleMinutes: 5
      postScaleMinutes: 10
    - eventType: "RaidBossSpawn"
      targetMicroservice: "raid-instance-manager"
      defaultReplicas: 2
      desiredReplicas: 15
      preScaleMinutes: 3
      postScaleMinutes: 5
```

Apply the CR:
```bash
kubectl apply -f config/samples/game_v1_gameeventscalerule.yaml
```

Restart the operator (if needed):
```bash
kubectl rollout restart deployment/k8s-operator-controller-manager -n k8s-operator-system
```

### 6. Operator Logs & Monitoring

```bash
# View operator logs
kubectl logs -n k8s-operator-system -l control-plane=controller-manager -f

# Check GameEventScaleRule status
kubectl get gameeventscalerules -A
kubectl describe gameeventscalerule main-game-event-scaler -n default

# Check deployments and pods
kubectl get deployments
kubectl get pods
```

---

## Cleanup

```bash
# Remove test deployments
kubectl delete deployment pvp-battle-service
kubectl delete deployment raid-instance-manager
# Or
kubectl delete -f test-deployments.yaml

# Remove test server
kubectl delete -f test-server/k8s-deployment.yaml

# Remove operator
make undeploy

# Remove CRDs
make uninstall

# Remove Kind cluster
kind delete cluster --name k8s-operator-test
```

---

## Production Deployment

```bash
# Build and push image
make docker-buildx IMG=your-registry/game-scaler-operator:latest
make docker-push IMG=your-registry/game-scaler-operator:latest

# Deploy to cluster
make deploy IMG=your-registry/game-scaler-operator:latest
```

---

## Project Structure

```
.
├── api/                    # API definitions (CRD)
├── cmd/                    # Operator entry point
├── config/                 # Kubernetes manifests
│   ├── crd/               # Custom Resource Definitions
│   ├── samples/           # CR examples
│   └── default/           # Operator deployment
├── internal/              # Internal logic
│   └── controller/        # Controllers
├── test-server/           # Test HTTP server
├── test-deployments.yaml  # Test Deployments for local testing
└── Makefile               # Build and deploy commands
```

---

## Troubleshooting

**ImagePullBackOff / ErrImagePull**
- Make sure you loaded the image into Kind:
  ```bash
  kind load docker-image <image-name>:latest --name k8s-operator-test
  ```
- Set `imagePullPolicy: IfNotPresent` in your deployments.

**DNS not resolving**
- Use full DNS name or IP in your CR:
  ```yaml
  eventEndpointURL: "http://game-event-api.default.svc.cluster.local/api/events"
  ```

**Operator not seeing CR changes**
- Restart the operator:
  ```bash
  kubectl rollout restart deployment/k8s-operator-controller-manager -n k8s-operator-system
  ```

**"Deployment not found for scaling rule"**
- Create test deployments that match `targetMicroservice` names:
  ```bash
  kubectl create deployment pvp-battle-service --image=nginx:alpine --replicas=3
  kubectl create deployment raid-instance-manager --image=nginx:alpine --replicas=2
  ```

**Test server changes not applied**
- After modifying test server code, rebuild and redeploy:
  ```bash
  cd test-server && docker build -t game-event-api:latest . && cd ..
  kind load docker-image game-event-api:latest --name k8s-operator-test
  kubectl rollout restart deployment/game-event-api
  ```

---

## License

[Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)

