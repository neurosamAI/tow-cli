# Tow Plugins (35)

> Created by [Murry Jeong (comchangs)](https://github.com/comchangs) · Supported by [neurosam.AI](https://neurosam.ai)

YAML-based module handler plugins for infrastructure services. Add support for any service without writing Go code.

## How It Works

```
./plugins/              # Project-level (committed to git)
~/.tow/plugins/         # Global (personal, all projects)
```

Tow automatically loads all `.yaml` files from these directories at startup.

```yaml
# tow.yaml — just set the type to the plugin name
modules:
  my-db:
    type: postgresql
    port: 5432
```

---

## Databases (6)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [mysql](mysql.yaml) | MySQL relational database | 8.4.3 | 3306 | TCP |
| [postgresql](postgresql.yaml) | PostgreSQL relational database | 16.6 | 5432 | TCP |
| [mariadb](mariadb.yaml) | MariaDB relational database | 11.4.4 | 3306 | TCP |
| [mongodb](mongodb.yaml) | MongoDB document database | 7.0.8 | 27017 | Command |
| [clickhouse](clickhouse.yaml) | ClickHouse analytics database | 24.12.3 | 8123 | HTTP `/ping` |
| [influxdb](influxdb.yaml) | InfluxDB time-series database | 2.7.10 | 8086 | HTTP `/health` |

<details>
<summary>Example: PostgreSQL + MongoDB</summary>

```yaml
modules:
  main-db:
    type: postgresql
    port: 5432
    data_dirs: [pgdata]
    health_check:
      type: tcp
      timeout: 30

  doc-store:
    type: mongodb
    port: 27017
    data_dirs: [data/db, data/log]
```
</details>

## Message Brokers & Streaming (3)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [kafka](kafka.yaml) | Apache Kafka event streaming | 3.7.0 | 9092 | TCP |
| [rabbitmq](rabbitmq.yaml) | RabbitMQ message broker | 3.13.1 | 5672 | TCP |
| [zookeeper](zookeeper.yaml) | Apache ZooKeeper coordination | 3.9.2 | 2181 | TCP |

<details>
<summary>Example: Kafka + ZooKeeper cluster</summary>

```yaml
modules:
  zookeeper:
    type: zookeeper
    port: 2181
    data_dirs: [data]

  kafka:
    type: kafka
    port: 9092
    data_dirs: [data/kafka-logs]

environments:
  prod:
    servers:
      - number: 1
        host: ${BROKER_1}
        modules: [zookeeper, kafka]
      - number: 2
        host: ${BROKER_2}
        modules: [zookeeper, kafka]
      - number: 3
        host: ${BROKER_3}
        modules: [zookeeper, kafka]
```
</details>

## Kafka Ecosystem (3)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [kafka-connect](kafka-connect.yaml) | Kafka Connect distributed mode | 3.7.0 | 8083 | HTTP `/connectors` |
| [kafka-lag-exporter](kafka-lag-exporter.yaml) | Kafka consumer lag monitoring | 0.8.2 | 9999 | TCP |
| [kminion](kminion.yaml) | KMinion Kafka monitoring | 2.2.3 | 8080 | HTTP `/ready` |
| [cmak](cmak.yaml) | Cluster Manager for Apache Kafka | 3.0.0.6 | 9000 | HTTP |

<details>
<summary>Example: Full Kafka stack with monitoring</summary>

```yaml
modules:
  kafka:
    type: kafka
    port: 9092
  kafka-connect:
    type: kafka-connect
    port: 8083
  kafka-lag:
    type: kafka-lag-exporter
    port: 9999
  kafka-ui:
    type: cmak
    port: 9000
```
</details>

## Caching & Storage (4)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [redis](redis.yaml) | Redis in-memory data store | 7.2.4 | 6379 | Command (`redis-cli ping`) |
| [memcached](memcached.yaml) | Memcached distributed caching | 1.6.32 | 11211 | TCP |
| [minio](minio.yaml) | MinIO S3-compatible object storage | latest | 9000 | HTTP `/minio/health/live` |
| [etcd](etcd.yaml) | etcd distributed key-value store | 3.5.17 | 2379 | HTTP `/health` |

<details>
<summary>Example: Redis cluster</summary>

```yaml
modules:
  redis:
    type: redis
    port: 6379
    data_dirs: [data]

environments:
  prod:
    servers:
      - number: 1
        host: ${REDIS_1}
        modules: [redis]
      - number: 2
        host: ${REDIS_2}
        modules: [redis]
      - number: 3
        host: ${REDIS_3}
        modules: [redis]
```
</details>

## Monitoring & Observability (4)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [prometheus](prometheus.yaml) | Prometheus metrics collection | 2.51.1 | 9090 | HTTP `/-/healthy` |
| [grafana](grafana.yaml) | Grafana visualization dashboard | 10.4.1 | 3000 | HTTP `/api/health` |
| [node-exporter](node-exporter.yaml) | Prometheus node metrics exporter | 1.7.0 | 9100 | HTTP `/metrics` |
| [kibana](kibana.yaml) | Kibana Elasticsearch visualization | 8.17.0 | 5601 | HTTP `/api/status` |

<details>
<summary>Example: Full monitoring stack</summary>

```yaml
modules:
  prometheus:
    type: prometheus
    port: 9090
  grafana:
    type: grafana
    port: 3000
    data_dirs: [data]
  node-exporter:
    type: node-exporter
    port: 9100
```
</details>

## Log Management (5)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [elasticsearch](elasticsearch.yaml) | Elasticsearch search engine | 8.13.0 | 9200 | HTTP `/_cluster/health` |
| [logstash](logstash.yaml) | Logstash log pipeline | 8.17.0 | 9600 | HTTP |
| [fluentd](fluentd.yaml) | Fluentd log collector | 1.17.1 | 24224 | TCP |
| [loki](loki.yaml) | Grafana Loki log aggregation | 2.9.4 | 3100 | HTTP `/ready` |
| [promtail](promtail.yaml) | Grafana Promtail log shipper | 2.9.4 | 9080 | HTTP `/ready` |

<details>
<summary>Example: ELK vs Loki stack</summary>

```yaml
# Option A: ELK stack
modules:
  elasticsearch:
    type: elasticsearch
    port: 9200
    data_dirs: [es-data]
  logstash:
    type: logstash
    port: 9600
  kibana:
    type: kibana
    port: 5601

# Option B: Grafana Loki stack
modules:
  loki:
    type: loki
    port: 3100
    data_dirs: [loki-data]
  promtail:
    type: promtail
    port: 9080
  grafana:
    type: grafana
    port: 3000
```
</details>

## Web & Proxy (2)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [nginx](nginx.yaml) | Nginx web server / reverse proxy | 1.25.4 | 80 | HTTP |
| [haproxy](haproxy.yaml) | HAProxy load balancer | 3.0.7 | 80 | TCP |

## Security & Identity (3)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [vault](vault.yaml) | HashiCorp Vault secrets management | 1.18.3 | 8200 | HTTP `/v1/sys/health` |
| [consul](consul.yaml) | HashiCorp Consul service mesh | 1.20.2 | 8500 | HTTP `/v1/status/leader` |
| [keycloak](keycloak.yaml) | Keycloak identity / SSO | 26.0.7 | 8080 | HTTP `/health/ready` |

<details>
<summary>Example: Vault + Consul</summary>

```yaml
modules:
  vault:
    type: vault
    port: 8200
    data_dirs: [vault-data]
  consul:
    type: consul
    port: 8500
    data_dirs: [consul-data]
```
</details>

## CI/CD & DevOps (2)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [jenkins](jenkins.yaml) | Jenkins CI/CD server | 2.492 | 8080 | HTTP `/login` |
| [sonarqube](sonarqube.yaml) | SonarQube code quality | 10.7.0 | 9000 | HTTP `/api/system/status` |

## Data & Workflow (2)

| Plugin | Service | Default Ver. | Port | Health Check |
|--------|---------|:---:|:---:|:---:|
| [airflow](airflow.yaml) | Apache Airflow workflow orchestration | 2.10.4 | 8080 | HTTP `/health` |
| [superset](superset.yaml) | Apache Superset data visualization | 4.1.1 | 8088 | HTTP `/health` |

---

## Writing a Plugin

```yaml
name: myservice
description: My custom service
version: "1.0.0"
author: Your Name

package:
  default_version: "3.7.0"
  url_template: "https://example.com/releases/{{VERSION}}/myservice-{{VERSION}}.tar.gz"
  versions:
    "3.7.0": "https://example.com/releases/3.7.0/myservice-3.7.0.tar.gz"

start_cmd: "{{BASE_DIR}}/current/bin/myservice --port {{PORT}}"
stop_cmd: "kill $(cat {{BASE_DIR}}/myservice.pid) 2>/dev/null || true"
status_cmd: "curl -sf http://localhost:{{PORT}}/health"

artifact_path: "external-package/myservice.tar.gz"
package_includes: [bin/, config/, lib/]

health_check:
  type: http
  target: "http://localhost:{{PORT}}/health"
  timeout: 60

provision:
  packages: [libssl-dev]
  directories: ["{{BASE_DIR}}/data"]
  commands: ["sysctl -w vm.max_map_count=262144 2>/dev/null || true"]

data_dirs: [data]
log_file: "myservice.log"
```

## Contributing

1. Create a YAML file following the schema above
2. Test: `tow validate && tow provision && tow deploy`
3. Submit a PR to [neurosamAI/tow-cli](https://github.com/neurosamAI/tow-cli)
