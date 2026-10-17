package cfg

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"cs-im-server/pkg/consts"
	"vane-common/dbserver"
	"vane-common/httpserver"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/logger"
	"vane-common/pkg/mysqldb"
	"vane-common/server/pkg/role"
	"vane-common/server/serverconfig"
	"vane-common/telemetry"
)

const ServiceRoleName = role.ServiceRole(consts.ServiceName)

type ConfigRoot struct {
	Server   ServerConfig       `yaml:"server"`
	Log      logger.LogConfig   `yaml:"log"`
	Access   AccessConfig       `yaml:"access"`
	HTTP     HTTPListenerConfig `yaml:"http"`
	Database DatabaseConfig     `yaml:"database"`
}

type ServerConfig struct {
	Host        string          `yaml:"host"`
	Port        int             `yaml:"port"`
	DisableGRPC bool            `yaml:"disable_grpc"`
	Standalone  bool            `yaml:"standalone"`
	Telemetry   TelemetryConfig `yaml:"telemetry"`
}

type TelemetryConfig struct {
	Port               int               `yaml:"port"`
	DisableHealthCheck bool              `yaml:"disable_health_check"`
	DisablePrometheus  bool              `yaml:"disable_prometheus"`
	PushGateway        PushGatewayConfig `yaml:"push_gateway"`
}

type PushGatewayConfig struct {
	Enabled  bool   `yaml:"enabled"`
	URL      string `yaml:"url"`
	JobName  string `yaml:"job_name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	PushGap  int    `yaml:"push_gap"`
}

type AccessConfig struct {
	AccountToken string `yaml:"account_token"`
	RelayToken   string `yaml:"relay_token"`
	TenantID     string `yaml:"tenant_id"`
}

type HTTPListenerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

var appConfig *ConfigRoot

func GetDbConfig() dbserver.Db {
	dbRelation := make(map[model.DbSource]store.RelationStore)
	if appConfig != nil && appConfig.Database.DSN != "" {
		mysqlDB, err := mysqldb.NewMysqlDB(mysqldb.Config{DSN: appConfig.Database.DSN})
		if err != nil {
			panic(fmt.Sprintf("[cfg] init mysql db failed: %v\nDSN=%q", err, appConfig.Database.DSN))
		}
		dbRelation["main"] = store.RelationStore{DB: mysqlDB}
	}
	return dbserver.Db{
		DbRelation: dbRelation,
		DbStore:    make(map[model.DbSource]store.IKVStore),
	}
}

func GetConfig() *ConfigRoot {
	return appConfig
}

func Init(path string) error {
	cfg := defaultConfig()
	if path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(content, cfg); err != nil {
			return fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if content, err := os.ReadFile(consts.DefaultConfigPath); err == nil {
		if err := yaml.Unmarshal(content, cfg); err != nil {
			return fmt.Errorf("parse config %s: %w", consts.DefaultConfigPath, err)
		}
	}
	applyDefaults(cfg)
	appConfig = cfg
	injectEnv()
	return nil
}

func defaultConfig() *ConfigRoot {
	return &ConfigRoot{
		Server: ServerConfig{
			Host:        consts.DefaultServerHost,
			Port:        consts.DefaultServerPort,
			DisableGRPC: true,
			Standalone:  true,
			Telemetry: TelemetryConfig{
				PushGateway: PushGatewayConfig{JobName: consts.ServiceName, PushGap: 15},
			},
		},
		Log: logger.LogConfig{
			Name:          consts.ServiceName,
			Level:         -1,
			MaxSize:       100,
			MaxBackup:     10,
			MaxAge:        7,
			ConsoleEncode: true,
		},
		Access: AccessConfig{
			AccountToken: "dev-access-token",
			RelayToken:   "dev-relay-token",
			TenantID:     "dev-tenant",
		},
		HTTP: HTTPListenerConfig{
			Host: consts.DefaultServerHost,
			Port: consts.DefaultServerPort,
		},
	}
}

func applyDefaults(cfg *ConfigRoot) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = consts.DefaultServerHost
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = consts.DefaultServerPort
	}
	if cfg.HTTP.Host == "" {
		cfg.HTTP.Host = cfg.Server.Host
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = cfg.Server.Port
	}
	if cfg.Access.AccountToken == "" {
		cfg.Access.AccountToken = "dev-access-token"
	}
	if cfg.Access.RelayToken == "" {
		cfg.Access.RelayToken = "dev-relay-token"
	}
	if cfg.Access.TenantID == "" {
		cfg.Access.TenantID = "dev-tenant"
	}
}

func injectEnv() {
	if appConfig == nil {
		return
	}
	if host := os.Getenv("SERVER_HOST"); host != "" {
		appConfig.Server.Host = host
		appConfig.HTTP.Host = host
	}
	if value := os.Getenv("SERVER_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			logger.Errorf(context.Background(), "convert SERVER_PORT to int error: %v", err)
		} else {
			appConfig.HTTP.Port = port
		}
	}
	if value := os.Getenv("GRPC_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			logger.Errorf(context.Background(), "convert GRPC_PORT to int error: %v", err)
		} else {
			appConfig.Server.Port = port
		}
	}
	if value := os.Getenv("CS_IM_ACCOUNT_TOKEN"); value != "" {
		appConfig.Access.AccountToken = value
	}
	if value := os.Getenv("CS_IM_RELAY_TOKEN"); value != "" {
		appConfig.Access.RelayToken = value
	}
	if value := os.Getenv("CS_IM_TENANT_ID"); value != "" {
		appConfig.Access.TenantID = value
	}
	if value := os.Getenv("CS_IM_DB_DSN"); value != "" {
		appConfig.Database.DSN = value
	}
}

func GetServerConfig() serverconfig.Config {
	if appConfig == nil {
		return serverconfig.Config{}
	}
	return serverconfig.Config{
		Hosts:           []string{appConfig.Server.Host},
		Port:            appConfig.Server.Port,
		DisableGRPC:     appConfig.Server.DisableGRPC,
		Standalone:      appConfig.Server.Standalone,
		EnableTelemetry: appConfig.Server.Telemetry.Port != 0,
		TelemetryPort:   appConfig.Server.Telemetry.Port,
		Telemetry: telemetry.Config{
			DisableHealthCheck: appConfig.Server.Telemetry.DisableHealthCheck,
			DisablePrometheus:  appConfig.Server.Telemetry.DisablePrometheus,
			PushGateway: telemetry.PushGateway{
				Enabled:  appConfig.Server.Telemetry.PushGateway.Enabled,
				URL:      appConfig.Server.Telemetry.PushGateway.URL,
				JobName:  appConfig.Server.Telemetry.PushGateway.JobName,
				Username: appConfig.Server.Telemetry.PushGateway.Username,
				Password: appConfig.Server.Telemetry.PushGateway.Password,
				PushGap:  appConfig.Server.Telemetry.PushGateway.PushGap,
			},
		},
	}
}

func GetHTTPConfig() httpserver.Config {
	if appConfig == nil {
		return httpserver.Config{}
	}
	return httpserver.Config{
		Host:            appConfig.HTTP.Host,
		Port:            appConfig.HTTP.Port,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}
}

func GetHTTPListenAddr() string {
	if appConfig == nil {
		return net.JoinHostPort(consts.DefaultServerHost, strconv.Itoa(consts.DefaultServerPort))
	}
	return net.JoinHostPort(strings.TrimSpace(appConfig.HTTP.Host), strconv.Itoa(appConfig.HTTP.Port))
}
