package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

const applicationConfigPath = "application.yaml"

type Config struct {
	ServiceName   string        `yaml:"serviceName"`
	Environment   string        `yaml:"environment"`
	HTTP          HTTP          `yaml:"http"`
	GRPC          GRPC          `yaml:"grpc"`
	Database      Database      `yaml:"database"`
	Redis         Redis         `yaml:"redis"`
	MessageBroker MessageBroker `yaml:"messageBroker"`
}

type HTTP struct {
	Port           string   `yaml:"port"`
	GinMode        string   `yaml:"ginMode"`
	SwaggerEnabled bool     `yaml:"swaggerEnabled"`
	CORSOrigins    []string `yaml:"corsOrigins"`
}

type GRPC struct {
	Port string `yaml:"port"`
}

type Database struct {
	URL string `yaml:"url"`
}

type Redis struct {
	Addrs      []string `yaml:"addrs"`
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	DB         int      `yaml:"db"`
	MasterName string   `yaml:"masterName"`
}

type MessageBroker struct {
	Addrs    []string        `yaml:"addrs"`
	Consumer MessageConsumer `yaml:"consumer"`
	Queue    []MessageQueue  `yaml:"queue"`
}

type MessageQueue struct {
	Name             string                   `yaml:"name"`
	Durability       bool                     `yaml:"durability"`
	DeleteWhenUnused bool                     `yaml:"deleteWhenUnused"`
	Exclusive        bool                     `yaml:"exclusive"`
	NoWait           bool                     `yaml:"noWait"`
	Consumer         *MessageConsumerOverride `yaml:"consumer"`
}

type MessageConsumer struct {
	Tag            string `yaml:"tag"`
	AutoAck        bool   `yaml:"autoAck"`
	Exclusive      bool   `yaml:"exclusive"`
	NoLocal        bool   `yaml:"noLocal"`
	NoWait         bool   `yaml:"noWait"`
	RequeueOnError bool   `yaml:"requeueOnError"`
}

type MessageConsumerOverride struct {
	Tag            string `yaml:"tag"`
	AutoAck        *bool  `yaml:"autoAck"`
	Exclusive      *bool  `yaml:"exclusive"`
	NoLocal        *bool  `yaml:"noLocal"`
	NoWait         *bool  `yaml:"noWait"`
	RequeueOnError *bool  `yaml:"requeueOnError"`
}

func (c MessageBroker) ConsumerFor(queue MessageQueue) MessageConsumer {
	consumer := c.Consumer
	if queue.Consumer == nil {
		return consumer
	}
	override := queue.Consumer
	if override.Tag != "" {
		consumer.Tag = override.Tag
	}
	if override.AutoAck != nil {
		consumer.AutoAck = *override.AutoAck
	}
	if override.Exclusive != nil {
		consumer.Exclusive = *override.Exclusive
	}
	if override.NoLocal != nil {
		consumer.NoLocal = *override.NoLocal
	}
	if override.NoWait != nil {
		consumer.NoWait = *override.NoWait
	}
	if override.RequeueOnError != nil {
		consumer.RequeueOnError = *override.RequeueOnError
	}
	return consumer
}

func Load() (*Config, error) {
	return load(applicationConfigPath)
}

func load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := Config{
		Environment: "release",
		HTTP: HTTP{
			Port:    "8080",
			GinMode: "release",
		},
		GRPC: GRPC{Port: "8081"},
		MessageBroker: MessageBroker{
			Consumer: MessageConsumer{RequeueOnError: true},
		},
	}
	if err := yaml.UnmarshalWithOptions(data, &cfg, yaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c Config) validate() error {
	if strings.TrimSpace(c.ServiceName) == "" {
		return fmt.Errorf("serviceName is required")
	}
	if strings.TrimSpace(c.Database.URL) == "" {
		return fmt.Errorf("database.url is required")
	}
	if err := validateAddresses("redis.addrs", c.Redis.Addrs); err != nil {
		return err
	}
	if err := validateAddresses("messageBroker.addrs", c.MessageBroker.Addrs); err != nil {
		return err
	}
	if len(c.MessageBroker.Queue) == 0 {
		return fmt.Errorf("messageBroker.queue must contain at least one queue")
	}
	queueNames := make(map[string]struct{}, len(c.MessageBroker.Queue))
	for i, queue := range c.MessageBroker.Queue {
		name := strings.TrimSpace(queue.Name)
		if name == "" {
			return fmt.Errorf("messageBroker.queue[%d].name is required", i)
		}
		if _, exists := queueNames[name]; exists {
			return fmt.Errorf("messageBroker.queue[%d].name duplicates %q", i, name)
		}
		queueNames[name] = struct{}{}
	}
	return nil
}

func validateAddresses(field string, addresses []string) error {
	if len(addresses) == 0 {
		return fmt.Errorf("%s must contain at least one address", field)
	}
	for i, address := range addresses {
		if strings.TrimSpace(address) == "" {
			return fmt.Errorf("%s[%d] must not be empty", field, i)
		}
	}
	return nil
}
