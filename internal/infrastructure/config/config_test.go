package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	path := writeConfig(t, `
serviceName: policy-service
http:
  swaggerEnabled: true
  corsOrigins:
    - http://localhost:3000
    - https://cms.example
database:
  url: postgres://test
redis:
  addrs:
    - localhost:6379
  username: redis-user
  password: redis-password
  db: 2
  masterName: primary
messageBroker:
  addrs:
    - amqp://guest:guest@localhost:5672/
  consumer:
    tag: default-consumer
    autoAck: true
    noWait: true
    requeueOnError: true
  queue:
    - name: reservations
      durability: true
      deleteWhenUnused: false
      exclusive: false
      noWait: true
      consumer:
        tag: reservations-consumer
        autoAck: false
        requeueOnError: false
`)

	cfg, err := load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Environment != "release" || cfg.HTTP.Port != "8080" || cfg.HTTP.GinMode != "release" || cfg.GRPC.Port != "8081" {
		t.Fatalf("defaults not applied: %#v", cfg)
	}
	if !cfg.HTTP.SwaggerEnabled || len(cfg.HTTP.CORSOrigins) != 2 || cfg.HTTP.CORSOrigins[1] != "https://cms.example" {
		t.Fatalf("HTTP config = %#v", cfg.HTTP)
	}
	if cfg.Redis.DB != 2 || cfg.Redis.MasterName != "primary" || cfg.Redis.Password != "redis-password" {
		t.Fatalf("Redis config = %#v", cfg.Redis)
	}
	if len(cfg.MessageBroker.Queue) != 1 {
		t.Fatalf("message broker queues = %#v", cfg.MessageBroker.Queue)
	}
	queue := cfg.MessageBroker.Queue[0]
	if queue.Name != "reservations" || !queue.Durability || queue.DeleteWhenUnused || queue.Exclusive || !queue.NoWait {
		t.Fatalf("queue config = %#v", queue)
	}
	consumer := cfg.MessageBroker.ConsumerFor(queue)
	if consumer.Tag != "reservations-consumer" || consumer.AutoAck || !consumer.NoWait || consumer.RequeueOnError {
		t.Fatalf("consumer config = %#v", consumer)
	}
}

func TestLoadYAMLRejectsInvalidConfig(t *testing.T) {
	valid := `
serviceName: policy-service
database:
  url: postgres://test
redis:
  addrs: [localhost:6379]
messageBroker:
  addrs: [amqp://guest:guest@localhost:5672/]
  queue:
    - name: reservations
`

	for name, content := range map[string]string{
		"malformed YAML":         "serviceName: [",
		"unknown field":          valid + "unknown: true\n",
		"missing service name":   strings.Replace(valid, "serviceName: policy-service\n", "", 1),
		"missing Redis address":  strings.Replace(valid, "  addrs: [localhost:6379]\n", "", 1),
		"missing broker address": strings.Replace(valid, "  addrs: [amqp://guest:guest@localhost:5672/]\n", "", 1),
		"missing queue":          strings.Replace(valid, "  queue:\n    - name: reservations\n", "", 1),
		"missing queue name":     strings.Replace(valid, "    - name: reservations\n", "    - durability: true\n", 1),
		"duplicate queue name":   valid + "    - name: reservations\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := load(writeConfig(t, content)); err == nil {
				t.Fatal("load() error = nil")
			}
		})
	}
}

func TestApplicationYAMLFiles(t *testing.T) {
	for _, path := range []string{"../../../application.example.yaml", "../../../e2e/application.yaml"} {
		t.Run(path, func(t *testing.T) {
			cfg, err := load(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.MessageBroker.Queue) != 1 || cfg.MessageBroker.Queue[0].Name != "usage-releases" || !cfg.MessageBroker.Queue[0].Durability {
				t.Fatalf("message broker queues = %#v", cfg.MessageBroker.Queue)
			}
		})
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "application.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
