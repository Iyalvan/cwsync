package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"cwsync/internal/logger"
	"cwsync/internal/offsetstore"
	"cwsync/internal/tailer"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/hashicorp/consul/api"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Consul                 ConsulConfig    `yaml:"consul"`
	DynamoDB               DynamoDBConfig  `yaml:"dynamodb"`
	File                   FileStoreConfig `yaml:"file"`
	OffsetStore            string          `yaml:"offset_store"` // default store: "file", "consul" or "dynamodb"
	AWSRegion              string          `yaml:"aws_region"`
	AWSProfile             string          `yaml:"aws_profile"`
	AWSRoleARN             string          `yaml:"aws_role_arn"`
	AWSAccessKey           string          `yaml:"aws_access_key"`
	AWSSecretKey           string          `yaml:"aws_secret_key"`
	Services               []ServiceConfig `yaml:"services"`
	OffsetFallbackDuration time.Duration   `yaml:"offset_fallback_duration"`
}

type ConsulConfig struct {
	Address string `yaml:"address"`
	Token   string `yaml:"token"`
}

type DynamoDBConfig struct {
	TableName string `yaml:"table_name"`
}

type FileStoreConfig struct {
	Path string `yaml:"path"`
}

type ServiceConfig struct {
	Name         string      `yaml:"name"`
	ConsulKVPath string      `yaml:"consul_kv_path"`
	LogConfigs   []LogConfig `yaml:"log_configs"`
	Destination  Destination `yaml:"destination"`
	OffsetStore  string      `yaml:"offset_store"` // "consul" (default) or "dynamodb"
}

type LogConfig struct {
	LogGroupName    string `yaml:"log_group_name"`
	LogStreamPrefix string `yaml:"log_stream_prefix"`
}

type Destination struct {
	Type     string `yaml:"type"`
	FilePath string `yaml:"file_path"`
	FileName string `yaml:"file_name"`
}

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	config := loadConfig(configPath)
	applyEnvOverrides(&config)
	sess := createAWSSession(config)
	// Lazily initialize stores/clients only if needed
	var consulClient *api.Client
	var dynamoClient *dynamodb.DynamoDB
	var fileStore *offsetstore.FileOffsetStore
	// Determine which stores are needed across services
	storesNeeded := map[string]bool{}
	for _, svc := range config.Services {
		st := strings.ToLower(strings.TrimSpace(svc.OffsetStore))
		if st == "" {
			st = strings.ToLower(strings.TrimSpace(config.OffsetStore))
		}
		if st == "" {
			st = "file"
		}
		storesNeeded[st] = true
	}
	if storesNeeded["file"] {
		filePath := config.File.Path
		if strings.TrimSpace(filePath) == "" {
			if exe, err := os.Executable(); err == nil {
				filePath = filepath.Join(filepath.Dir(exe), "offsetstore.json")
			} else {
				filePath = "offsetstore.json"
			}
		}
		fileStore = offsetstore.NewFileOffsetStore(filePath)
	}
	if storesNeeded["consul"] {
		consulClient = setupConsulClient(config.Consul)
	}
	if storesNeeded["dynamodb"] {
		dynamoClient = dynamodb.New(sess)
	}
	OffsetFallbackDuration := config.OffsetFallbackDuration

	for _, service := range config.Services {
		cwLogs := cloudwatchlogs.New(sess)

		for _, logConfig := range service.LogConfigs {
			logStreams, err := tailer.ListLogStreams(cwLogs, logConfig.LogGroupName, logConfig.LogStreamPrefix)
			if err != nil {
				logger.Fatal.Fatalf("failed to list log streams for %s: %v", service.Name, err)
			}

			// pick store for this service
			storeType := strings.ToLower(strings.TrimSpace(service.OffsetStore))
			if storeType == "" {
				storeType = strings.ToLower(strings.TrimSpace(config.OffsetStore))
			}
			if storeType == "" {
				storeType = "file"
			}
			var offsetStore offsetstore.OffsetStore
			switch storeType {
			case "file":
				if fileStore == nil {
					logger.Fatal.Fatalf("file offset store requested but not initialized")
				}
				offsetStore = fileStore
			case "dynamodb":
				if dynamoClient == nil {
					logger.Fatal.Fatalf("dynamodb offset store requested but dynamodb client not initialized")
				}
				if strings.TrimSpace(config.DynamoDB.TableName) == "" {
					logger.Fatal.Fatalf("dynamodb offset store requires 'dynamodb.table_name' in config or env")
				}
				offsetStore = &offsetstore.DynamoDBOffsetStore{Client: dynamoClient, TableName: config.DynamoDB.TableName}
			case "consul":
				fallthrough
			default:
				if consulClient == nil {
					logger.Fatal.Fatalf("consul offset store requested but consul client not initialized")
				}
				offsetStore = &offsetstore.ConsulOffsetStore{Client: consulClient}
			}

			for _, stream := range logStreams {
				go tailer.TailLogStream(cwLogs, service.ConsulKVPath, logConfig.LogGroupName, stream, offsetStore, OffsetFallbackDuration)
			}
		}
	}

	select {}
}

func loadConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Fatal.Fatalf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		logger.Fatal.Fatalf("failed to unmarshal config file: %v", err)
	}
	return config
}

func setupConsulClient(consulConfig ConsulConfig) *api.Client {
	config := api.DefaultConfig()
	config.Address = consulConfig.Address
	config.Token = consulConfig.Token
	client, err := api.NewClient(config)
	if err != nil {
		logger.Fatal.Fatalf("failed to create Consul client: %v", err)
	}
	return client
}

func createAWSSession(config Config) *session.Session {
	sessOptions := session.Options{
		Config: aws.Config{
			Region: aws.String(config.AWSRegion),
		},
	}
	// I used profile for local testing
	if config.AWSProfile != "" {
		sessOptions.Profile = config.AWSProfile
	} else if config.AWSRoleARN != "" {
		sess := session.Must(session.NewSession(&sessOptions.Config))
		creds := stscreds.NewCredentials(sess, config.AWSRoleARN)
		sessOptions.Config.Credentials = creds
	} else if config.AWSAccessKey != "" && config.AWSSecretKey != "" {
		sessOptions.Config.Credentials = credentials.NewStaticCredentials(
			config.AWSAccessKey,
			config.AWSSecretKey, "",
		)
	} else {
		sessOptions.SharedConfigState = session.SharedConfigEnable
	}

	return session.Must(session.NewSessionWithOptions(sessOptions))
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("CWSYNC_OFFSET_STORE"); v != "" {
		cfg.OffsetStore = v
	}
	if v := os.Getenv("CWSYNC_OFFSET_FILE"); v != "" {
		cfg.File.Path = v
	}
	if v := os.Getenv("CWSYNC_AWS_REGION"); v != "" {
		cfg.AWSRegion = v
	}
	if v := os.Getenv("CWSYNC_AWS_PROFILE"); v != "" {
		cfg.AWSProfile = v
	}
	if v := os.Getenv("CWSYNC_AWS_ROLE_ARN"); v != "" {
		cfg.AWSRoleARN = v
	}
	if v := os.Getenv("CWSYNC_AWS_ACCESS_KEY"); v != "" {
		cfg.AWSAccessKey = v
	}
	if v := os.Getenv("CWSYNC_AWS_SECRET_KEY"); v != "" {
		cfg.AWSSecretKey = v
	}
	if v := os.Getenv("CWSYNC_CONSUL_ADDR"); v != "" {
		cfg.Consul.Address = v
	}
	if v := os.Getenv("CWSYNC_CONSUL_TOKEN"); v != "" {
		cfg.Consul.Token = v
	}
	if v := os.Getenv("CWSYNC_DYNAMODB_TABLE"); v != "" {
		cfg.DynamoDB.TableName = v
	}
	if v := os.Getenv("CWSYNC_OFFSET_FALLBACK_DURATION"); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			cfg.OffsetFallbackDuration = dur
		} else {
			logger.Error.Printf("Invalid CWSYNC_OFFSET_FALLBACK_DURATION '%s': %v", v, err)
		}
	}
}
