# 06 - 基础设施

## 1. 基础设施概览

### 1.1 组件清单

| 组件 | 用途 | 端口 | 部署方式 |
|------|------|------|---------|
| Qdrant | 向量数据库 | 6333 | Docker |
| MongoDB | 文档数据库 | 27017 | Docker |
| Redis | 缓存 | 6379 | Docker |
| SQLite | 元数据 | - | 嵌入式 |
| OpenAI API | LLM | - | 外部服务 |
| Embedding API | 向量化 | - | 外部服务 |

### 1.2 架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Kubernetes Cluster                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                          Ingress Nginx                              │   │
│  │                           (Load Balancer)                           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Gateway Service                              │   │
│  │                      (Deployment: 2 replicas)                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        Service Mesh (Istio)                          │   │
│  │                                                                     │   │
│  │   ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐    │   │
│  │   │  Chat   │ │Knowledge│ │ Memory  │ │   A2A   │ │   MCP   │    │   │
│  │   │   (2)   │ │   (2)   │ │   (2)   │ │   (2)   │ │   (2)   │    │   │
│  │   └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘    │   │
│  │                                                                     │   │
│  │   ┌─────────┐                                                     │   │
│  │   │ Harness │                                                     │   │
│  │   │   (2)   │                                                     │   │
│  │   └─────────┘                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                          Data Layer                                 │   │
│  │                                                                     │   │
│  │   ┌─────────┐ ┌─────────┐ ┌─────────┐                             │   │
│  │   │ Qdrant  │ │ MongoDB │ │  Redis  │                             │   │
│  │   │ Stateful│ │ Stateful│ │ Stateful│                             │   │
│  │   └─────────┘ └─────────┘ └─────────┘                             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Docker Compose 配置

### 2.1 开发环境

**文件：`docker/docker-compose.dev.yaml`**

```yaml
version: '3.8'

services:
  # API 网关
  gateway:
    build:
      context: ../services/gateway
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ../services/gateway/config.yaml:/app/config.yaml
    depends_on:
      - chat-service
      - knowledge-service
      - memory-service
      - a2a-service
      - mcp-service
      - harness-service
      - redis
    networks:
      - agent-network

  # 对话服务
  chat-service:
    build:
      context: ../services/chat-service
      dockerfile: Dockerfile
    ports:
      - "50001:50001"
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ../services/chat-service/config.yaml:/app/config.yaml
      - chat-data:/app/data
    depends_on:
      - memory-service
      - mcp-service
      - harness-service
      - qdrant
      - mongodb
    networks:
      - agent-network

  # 知识库服务
  knowledge-service:
    build:
      context: ../services/knowledge-service
      dockerfile: Dockerfile
    ports:
      - "50002:50002"
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ../services/knowledge-service/config.yaml:/app/config.yaml
      - knowledge-data:/app/data
      - upload-temp:/app/uploads
    depends_on:
      - qdrant
      - mongodb
      - redis
    networks:
      - agent-network

  # 记忆服务
  memory-service:
    build:
      context: ../services/memory-service
      dockerfile: Dockerfile
    ports:
      - "50003:50003"
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ../services/memory-service/config.yaml:/app/config.yaml
      - memory-data:/app/data
    depends_on:
      - qdrant
    networks:
      - agent-network

  # A2A 服务
  a2a-service:
    build:
      context: ../services/a2a-service
      dockerfile: Dockerfile
    ports:
      - "50004:50004"
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ../services/a2a-service/config.yaml:/app/config.yaml
      - a2a-data:/app/data
    networks:
      - agent-network

  # MCP 服务
  mcp-service:
    build:
      context: ../services/mcp-service
      dockerfile: Dockerfile
    ports:
      - "50005:50005"
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ../services/mcp-service/config.yaml:/app/config.yaml
    networks:
      - agent-network

  # Harness 服务
  harness-service:
    build:
      context: ../services/harness-service
      dockerfile: Dockerfile
    ports:
      - "50006:50006"
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ../services/harness-service/config.yaml:/app/config.yaml
      - harness-data:/app/data
    networks:
      - agent-network

  # Qdrant 向量数据库
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - qdrant-data:/qdrant/storage
    networks:
      - agent-network

  # MongoDB
  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"
    environment:
      - MONGO_INITDB_ROOT_USERNAME=admin
      - MONGO_INITDB_ROOT_PASSWORD=admin123
    volumes:
      - mongodb-data:/data/db
    networks:
      - agent-network

  # Redis
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    networks:
      - agent-network

  # 前端 (开发模式)
  frontend:
    build:
      context: ../frontend
      dockerfile: Dockerfile.dev
    ports:
      - "5173:5173"
    volumes:
      - ../frontend:/app
      - /app/node_modules
    environment:
      - VITE_API_URL=http://localhost:8080
    networks:
      - agent-network

networks:
  agent-network:
    driver: bridge

volumes:
  chat-data:
  knowledge-data:
  memory-data:
  a2a-data:
  harness-data:
  qdrant-data:
  mongodb-data:
  redis-data:
  upload-temp:
```

### 2.2 生产环境

**文件：`docker/docker-compose.yaml`**

```yaml
version: '3.8'

services:
  gateway:
    image: agent-platform/gateway:${VERSION:-latest}
    ports:
      - "8080:8080"
    environment:
      - CONFIG_PATH=/app/config.yaml
      - GIN_MODE=release
    configs:
      - gateway-config
    depends_on:
      - chat-service
      - knowledge-service
      - redis
    networks:
      - agent-network
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '1'
          memory: 512M

  chat-service:
    image: agent-platform/chat-service:${VERSION:-latest}
    configs:
      - chat-config
    networks:
      - agent-network
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 1G

  knowledge-service:
    image: agent-platform/knowledge-service:${VERSION:-latest}
    configs:
      - knowledge-config
    networks:
      - agent-network
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 1G

  memory-service:
    image: agent-platform/memory-service:${VERSION:-latest}
    configs:
      - memory-config
    networks:
      - agent-network
    deploy:
      replicas: 2

  a2a-service:
    image: agent-platform/a2a-service:${VERSION:-latest}
    configs:
      - a2a-config
    networks:
      - agent-network
    deploy:
      replicas: 2

  mcp-service:
    image: agent-platform/mcp-service:${VERSION:-latest}
    configs:
      - mcp-config
    networks:
      - agent-network
    deploy:
      replicas: 2

  harness-service:
    image: agent-platform/harness-service:${VERSION:-latest}
    configs:
      - harness-config
    networks:
      - agent-network
    deploy:
      replicas: 2

  qdrant:
    image: qdrant/qdrant:latest
    volumes:
      - qdrant-data:/qdrant/storage
    networks:
      - agent-network
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G

  mongodb:
    image: mongo:7
    environment:
      - MONGO_INITDB_ROOT_USERNAME=${MONGO_USER}
      - MONGO_INITDB_ROOT_PASSWORD=${MONGO_PASSWORD}
    volumes:
      - mongodb-data:/data/db
    networks:
      - agent-network
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
    networks:
      - agent-network

  frontend:
    image: agent-platform/frontend:${VERSION:-latest}
    ports:
      - "80:80"
    networks:
      - agent-network

configs:
  gateway-config:
    file: ../services/gateway/config.prod.yaml
  chat-config:
    file: ../services/chat-service/config.prod.yaml
  knowledge-config:
    file: ../services/knowledge-service/config.prod.yaml
  memory-config:
    file: ../services/memory-service/config.prod.yaml
  a2a-config:
    file: ../services/a2a-service/config.prod.yaml
  mcp-config:
    file: ../services/mcp-service/config.prod.yaml
  harness-config:
    file: ../services/harness-service/config.prod.yaml

networks:
  agent-network:
    driver: bridge

volumes:
  qdrant-data:
  mongodb-data:
  redis-data:
```

---

## 3. Kubernetes 配置

### 3.1 Namespace

**文件：`k8s/namespace.yaml`**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: agent-platform
```

### 3.2 ConfigMap

**文件：`k8s/configmap.yaml`**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: agent-platform-config
  namespace: agent-platform
data:
  QDRANT_URL: "http://qdrant:6333"
  MONGODB_URL: "mongodb://mongodb:27017"
  REDIS_URL: "redis://redis:6379"
```

### 3.3 Secret

**文件：`k8s/secret.yaml`**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: agent-platform-secret
  namespace: agent-platform
type: Opaque
stringData:
  OPENAI_API_KEY: "your-api-key"
  MONGO_USERNAME: "admin"
  MONGO_PASSWORD: "admin123"
  JWT_SECRET: "your-jwt-secret"
```

### 3.4 Deployments

**文件：`k8s/deployments.yaml`**

```yaml
# Chat Service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: chat-service
  namespace: agent-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: chat-service
  template:
    metadata:
      labels:
        app: chat-service
    spec:
      containers:
      - name: chat-service
        image: agent-platform/chat-service:latest
        ports:
        - containerPort: 50001
        envFrom:
        - configMapRef:
            name: agent-platform-config
        - secretRef:
            name: agent-platform-secret
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          grpc:
            port: 50001
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          grpc:
            port: 50001
          initialDelaySeconds: 5
          periodSeconds: 5

---
# Knowledge Service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: knowledge-service
  namespace: agent-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: knowledge-service
  template:
    metadata:
      labels:
        app: knowledge-service
    spec:
      containers:
      - name: knowledge-service
        image: agent-platform/knowledge-service:latest
        ports:
        - containerPort: 50002
        envFrom:
        - configMapRef:
            name: agent-platform-config
        - secretRef:
            name: agent-platform-secret
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1"
        volumeMounts:
        - name: upload-temp
          mountPath: /app/uploads
      volumes:
      - name: upload-temp
        emptyDir: {}

---
# Memory Service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: memory-service
  namespace: agent-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: memory-service
  template:
    metadata:
      labels:
        app: memory-service
    spec:
      containers:
      - name: memory-service
        image: agent-platform/memory-service:latest
        ports:
        - containerPort: 50003
        envFrom:
        - configMapRef:
            name: agent-platform-config
        - secretRef:
            name: agent-platform-secret

---
# A2A Service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: a2a-service
  namespace: agent-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: a2a-service
  template:
    metadata:
      labels:
        app: a2a-service
    spec:
      containers:
      - name: a2a-service
        image: agent-platform/a2a-service:latest
        ports:
        - containerPort: 50004
        envFrom:
        - configMapRef:
            name: agent-platform-config
        - secretRef:
            name: agent-platform-secret

---
# MCP Service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-service
  namespace: agent-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mcp-service
  template:
    metadata:
      labels:
        app: mcp-service
    spec:
      containers:
      - name: mcp-service
        image: agent-platform/mcp-service:latest
        ports:
        - containerPort: 50005
        envFrom:
        - configMapRef:
            name: agent-platform-config
        - secretRef:
            name: agent-platform-secret

---
# Harness Service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: harness-service
  namespace: agent-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: harness-service
  template:
    metadata:
      labels:
        app: harness-service
    spec:
      containers:
      - name: harness-service
        image: agent-platform/harness-service:latest
        ports:
        - containerPort: 50006
        envFrom:
        - configMapRef:
            name: agent-platform-config
        - secretRef:
            name: agent-platform-secret

---
# Gateway
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: agent-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
    spec:
      containers:
      - name: gateway
        image: agent-platform/gateway:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: agent-platform-config
        - secretRef:
            name: agent-platform-secret
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "250m"
```

### 3.5 Services

**文件：`k8s/services.yaml`**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: chat-service
  namespace: agent-platform
spec:
  selector:
    app: chat-service
  ports:
  - port: 50001
    targetPort: 50001

---
apiVersion: v1
kind: Service
metadata:
  name: knowledge-service
  namespace: agent-platform
spec:
  selector:
    app: knowledge-service
  ports:
  - port: 50002
    targetPort: 50002

---
apiVersion: v1
kind: Service
metadata:
  name: memory-service
  namespace: agent-platform
spec:
  selector:
    app: memory-service
  ports:
  - port: 50003
    targetPort: 50003

---
apiVersion: v1
kind: Service
metadata:
  name: a2a-service
  namespace: agent-platform
spec:
  selector:
    app: a2a-service
  ports:
  - port: 50004
    targetPort: 50004

---
apiVersion: v1
kind: Service
metadata:
  name: mcp-service
  namespace: agent-platform
spec:
  selector:
    app: mcp-service
  ports:
  - port: 50005
    targetPort: 50005

---
apiVersion: v1
kind: Service
metadata:
  name: harness-service
  namespace: agent-platform
spec:
  selector:
    app: harness-service
  ports:
  - port: 50006
    targetPort: 50006

---
apiVersion: v1
kind: Service
metadata:
  name: gateway
  namespace: agent-platform
spec:
  selector:
    app: gateway
  ports:
  - port: 8080
    targetPort: 8080
  type: LoadBalancer
```

### 3.6 StatefulSets

**文件：`k8s/statefulsets.yaml`**

```yaml
# Qdrant
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: qdrant
  namespace: agent-platform
spec:
  serviceName: qdrant
  replicas: 1
  selector:
    matchLabels:
      app: qdrant
  template:
    metadata:
      labels:
        app: qdrant
    spec:
      containers:
      - name: qdrant
        image: qdrant/qdrant:latest
        ports:
        - containerPort: 6333
        volumeMounts:
        - name: qdrant-data
          mountPath: /qdrant/storage
  volumeClaimTemplates:
  - metadata:
      name: qdrant-data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 10Gi

---
# MongoDB
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mongodb
  namespace: agent-platform
spec:
  serviceName: mongodb
  replicas: 1
  selector:
    matchLabels:
      app: mongodb
  template:
    metadata:
      labels:
        app: mongodb
    spec:
      containers:
      - name: mongodb
        image: mongo:7
        ports:
        - containerPort: 27017
        env:
        - name: MONGO_INITDB_ROOT_USERNAME
          valueFrom:
            secretKeyRef:
              name: agent-platform-secret
              key: MONGO_USERNAME
        - name: MONGO_INITDB_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: agent-platform-secret
              key: MONGO_PASSWORD
        volumeMounts:
        - name: mongodb-data
          mountPath: /data/db
  volumeClaimTemplates:
  - metadata:
      name: mongodb-data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 10Gi

---
# Redis
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: agent-platform
spec:
  serviceName: redis
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command: ["redis-server", "--appendonly", "yes"]
        ports:
        - containerPort: 6379
        volumeMounts:
        - name: redis-data
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: redis-data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 5Gi
```

---

## 4. 配置文件

### 4.1 服务配置模板

**文件：`services/chat-service/config.yaml`**

```yaml
server:
  grpc_port: 50001
  http_port: 8081

database:
  sqlite:
    path: ./data/chat.db

llm:
  provider: openai  # openai / dashscope
  openai:
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    model: gpt-4
  dashscope:
    api_key: ${DASHSCOPE_API_KEY}
    model: qwen-turbo

services:
  memory:
    address: memory-service:50003
  mcp:
    address: mcp-service:50005
  harness:
    address: harness-service:50006

embedding:
  provider: openai
  model: text-embedding-3-small
  api_key: ${OPENAI_API_KEY}

agent:
  default_model: gpt-4
  max_tokens: 4096
  temperature: 0.7

logging:
  level: info
  format: json
```

**文件：`services/knowledge-service/config.yaml`**

```yaml
server:
  grpc_port: 50002

database:
  sqlite:
    path: ./data/knowledge.db

qdrant:
  url: http://qdrant:6333
  collection: documents

mongodb:
  url: mongodb://mongodb:27017
  database: agent_platform
  username: ${MONGO_USERNAME}
  password: ${MONGO_PASSWORD}

redis:
  url: redis://redis:6379

embedding:
  provider: openai
  model: text-embedding-3-small
  api_key: ${OPENAI_API_KEY}
  batch_size: 20

upload:
  temp_dir: ./uploads
  max_size: 50MB
  allowed_types:
    - pdf
    - docx
    - md
    - txt
    - json
    - csv

chunk:
  default_strategy: token
  default_size: 512
  default_overlap: 50

logging:
  level: info
  format: json
```

---

## 5. 监控配置

### 5.1 Prometheus 配置

**文件：`docker/prometheus/prometheus.yml`**

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'gateway'
    static_configs:
      - targets: ['gateway:8080']

  - job_name: 'chat-service'
    static_configs:
      - targets: ['chat-service:50001']

  - job_name: 'knowledge-service'
    static_configs:
      - targets: ['knowledge-service:50002']

  - job_name: 'memory-service'
    static_configs:
      - targets: ['memory-service:50003']

  - job_name: 'a2a-service'
    static_configs:
      - targets: ['a2a-service:50004']

  - job_name: 'mcp-service'
    static_configs:
      - targets: ['mcp-service:50005']

  - job_name: 'harness-service'
    static_configs:
      - targets: ['harness-service:50006']

  - job_name: 'qdrant'
    static_configs:
      - targets: ['qdrant:6333']

  - job_name: 'mongodb'
    static_configs:
      - targets: ['mongodb:27017']

  - job_name: 'redis'
    static_configs:
      - targets: ['redis:6379']
```

### 5.2 Grafana Dashboard

**文件：`docker/grafana/dashboards/agent-platform.json`**

```json
{
  "dashboard": {
    "title": "Agent Platform Overview",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{service}}"
          }
        ]
      },
      {
        "title": "Latency P99",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "{{service}}"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(http_requests_total{status=\"5xx\"}[5m])",
            "legendFormat": "{{service}}"
          }
        ]
      },
      {
        "title": "Token Usage",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(llm_tokens_total[5m])",
            "legendFormat": "{{model}}"
          }
        ]
      }
    ]
  }
}
```

---

## 6. CI/CD 配置

### 6.1 GitHub Actions

**文件：`.github/workflows/ci.yaml`**

```yaml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Generate protobuf
        run: make proto

      - name: Run tests
        run: go test -v ./...

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}

      - name: Build and push images
        run: |
          docker-compose -f docker/docker-compose.yaml build
          docker-compose -f docker/docker-compose.yaml push

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Deploy to Kubernetes
        run: |
          kubectl apply -f k8s/
```

---

## 7. Makefile

```makefile
.PHONY: proto build run-dev run-prod clean test docker-build docker-push

# 生成 protobuf
proto:
	@echo "Generating protobuf..."
	@for dir in common chat knowledge memory a2a mcp harness; do \
		protoc --go_out=./pkg/pb --go-grpc_out=./pkg/pb \
			--go_opt=paths=source_relative \
			--go-grpc_opt=paths=source_relative \
			proto/$$dir/*.proto; \
	done

# 构建所有服务
build:
	@echo "Building services..."
	@go build -o bin/gateway ./services/gateway/cmd
	@go build -o bin/chat-service ./services/chat-service/cmd
	@go build -o bin/knowledge-service ./services/knowledge-service/cmd
	@go build -o bin/memory-service ./services/memory-service/cmd
	@go build -o bin/a2a-service ./services/a2a-service/cmd
	@go build -o bin/mcp-service ./services/mcp-service/cmd
	@go build -o bin/harness-service ./services/harness-service/cmd

# 运行开发环境
run-dev:
	docker-compose -f docker/docker-compose.dev.yaml up -d

# 运行生产环境
run-prod:
	docker-compose -f docker/docker-compose.yaml up -d

# 停止
stop:
	docker-compose -f docker/docker-compose.dev.yaml down
	docker-compose -f docker/docker-compose.yaml down

# 清理
clean:
	rm -rf bin/
	rm -rf pkg/pb/
	docker-compose -f docker/docker-compose.dev.yaml down -v

# 测试
test:
	go test -v -race ./...

# Docker 构建
docker-build:
	docker-compose -f docker/docker-compose.yaml build

# Docker 推送
docker-push:
	docker-compose -f docker/docker-compose.yaml push

# Kubernetes 部署
k8s-deploy:
	kubectl apply -f k8s/

# Kubernetes 删除
k8s-delete:
	kubectl delete -f k8s/
```