# Game Event Scaler Operator

Kubernetes operator for automatic scaling of microservices based on game events. The operator monitors external event APIs and automatically scales corresponding Deployments in the Kubernetes cluster.

## Description

Game Event Scaler Operator is a Kubernetes operator that:
- Periodically polls external APIs to retrieve game event information
- Automatically scales microservices based on active events
- Supports various event types (PvP, Raid Boss, etc.)
- Provides detailed information about scaling status

## Local Testing

### Prerequisites

- Go 1.24+
- Docker
- kubectl
- Kind (Kubernetes in Docker) - for local cluster

### Installing Kind

```bash
# macOS
brew install kind

# Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.29.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

### Creating Local Cluster

```bash
# Create Kind cluster
kind create cluster --name k8s-operator-test

# Check cluster status
kubectl cluster-info --context kind-k8s-operator-test
```

### Setting up the Operator

1. **Install CRDs:**
```bash
make install
```

2. **Build Docker image for operator:**
```bash
make docker-build IMG=controller:latest
```

3. **Load image into Kind:**
```bash
kind load docker-image controller:latest --name k8s-operator-test
```

4. **Deploy operator:**
```bash
make deploy IMG=controller:latest
```

### Test API Server

A simple HTTP server has been created to simulate the external events API for testing the operator.

#### Test-server structure:

```
test-server/
├── main.go              # HTTP server with /api/events endpoint
├── Dockerfile           # Docker image for server
└── k8s-deployment.yaml  # Kubernetes manifests
```

#### Deploy test server:

```bash
# Build test server image
cd test-server
docker build -t game-event-api:latest .

# Load into Kind
cd ..
kind load docker-image game-event-api:latest --name k8s-operator-test

# Deploy server
kubectl apply -f test-server/k8s-deployment.yaml
```

#### Verify test server operation:

```bash
# Check pod status
kubectl get pods -l app=game-event-api

# Check service
kubectl get svc game-event-api

# Test API from within cluster
kubectl run -it --rm --restart=Never busybox --image=busybox:1.36 --namespace=default -- sh
# Inside the pod:
wget -qO- http://game-event-api.default.svc.cluster.local/api/events
```

### Configuring Custom Resource

1. **Update endpoint URL in CR:**
```yaml
# config/samples/game_v1_gameeventscalerule.yaml
spec:
  eventEndpointURL: "http://game-event-api.default.svc.cluster.local/api/events"
  # or use IP address:
  # eventEndpointURL: "http://10.96.108.198/api/events"
```

2. **Apply CR:**
```bash
kubectl apply -f config/samples/game_v1_gameeventscalerule.yaml
```

3. **Restart operator:**
```bash
kubectl rollout restart deployment/k8s-operator-controller-manager -n k8s-operator-system
```

### Monitoring and Diagnostics

#### View operator logs:

```bash
# Logs from all operator pods
kubectl logs -n k8s-operator-system -l app.kubernetes.io/name=k8s-operator -f

# Logs from specific pod
kubectl get pods -n k8s-operator-system
kubectl logs -n k8s-operator-system <pod-name>

# Last 50 lines of logs
kubectl logs -n k8s-operator-system -l app.kubernetes.io/name=k8s-operator --tail=50
```

#### Check resource status:

```bash
# GameEventScaleRule status
kubectl get gameeventscalerules -A
kubectl describe gameeventscalerule main-game-event-scaler -n default

# Operator pod status
kubectl get pods -n k8s-operator-system

# Test server status
kubectl get pods -l app=game-event-api
kubectl get svc game-event-api

# Cluster events
kubectl get events --all-namespaces --sort-by='.lastTimestamp'
```

#### Check scaling:

```bash
# View Deployments being scaled
kubectl get deployments -n default

# Detailed Deployment information
kubectl describe deployment <deployment-name> -n default
```

### Custom Resource Structure

```yaml
apiVersion: game.game.yourdomain.com/v1
kind: GameEventScaleRule
metadata:
  name: main-game-event-scaler
  namespace: default
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

### API Response Format

The test server returns JSON in this format:

```json
[
  {
    "eventType": "MassPvPEvent",
    "startTime": "2025-06-29T13:38:37Z",
    "endTime": "2025-06-29T14:08:37Z",
    "targetMicroservice": "pvp-battle-service"
  },
  {
    "eventType": "RaidBossSpawn",
    "startTime": "2025-06-29T13:53:37Z", 
    "endTime": "2025-06-29T14:23:37Z",
    "targetMicroservice": "raid-instance-manager"
  }
]
```

### Development

#### Local operator run (without Docker):

```bash
# Run operator locally (connects to Kind cluster)
make run
```

#### Testing changes:

```bash
# After code changes, rebuild image
make docker-build IMG=controller:latest
kind load docker-image controller:latest --name k8s-operator-test

# Restart operator
kubectl rollout restart deployment/k8s-operator-controller-manager -n k8s-operator-system
```

### Cleanup

```bash
# Remove test server
kubectl delete -f test-server/k8s-deployment.yaml

# Remove operator
make undeploy

# Remove CRDs
make uninstall

# Remove Kind cluster
kind delete cluster --name k8s-operator-test
```

## Production Deployment

### Build and publish image

```bash
# Build for multiple architectures
make docker-buildx IMG=your-registry/game-scaler-operator:latest

# Publish to registry
make docker-push IMG=your-registry/game-scaler-operator:latest
```

### Deploy to cluster

```bash
# Deploy with your image
make deploy IMG=your-registry/game-scaler-operator:latest
```

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
└── Makefile               # Build and deploy commands
```

## Troubleshooting

### Common issues:

1. **ImagePullBackOff** - image not found in Kind
   ```bash
   kind load docker-image <image-name>:latest --name k8s-operator-test
   ```

2. **DNS not resolving** - use full DNS name or IP
   ```yaml
   eventEndpointURL: "http://service-name.namespace.svc.cluster.local/api/events"
   ```

3. **Operator not seeing CR changes** - restart operator
   ```bash
   kubectl rollout restart deployment/k8s-operator-controller-manager -n k8s-operator-system
   ```

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

