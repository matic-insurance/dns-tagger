package pkg

import (
	"fmt"
	"reflect"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/sirupsen/logrus"
)

var Version = "unknown"

type Config struct {
	Mode           string
	APIServerURL   string
	KubeConfig     string
	RequestTimeout time.Duration
	Sources        []string
	Namespace      string
	Provider       string
	LogFormat      string
	LogLevel       string

	Apply            bool
	CurrentOwnerID   string
	PreviousOwnerIDs []string
	DNSZones         []string
}

var defaultConfig = &Config{
	Mode:           "owner",
	APIServerURL:   "",
	KubeConfig:     "",
	RequestTimeout: time.Second * 30,
	Sources:        nil,
	Namespace:      "",
	Provider:       "",
	LogFormat:      "text",
	LogLevel:       logrus.InfoLevel.String(),

	Apply:    false,
	DNSZones: []string{},
}

func NewConfig() *Config {
	return &Config{}
}

func (cfg *Config) String() string {
	// prevent logging of sensitive information
	temp := *cfg

	t := reflect.TypeOf(temp)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if val, ok := f.Tag.Lookup("secure"); ok && val == "yes" {
			if f.Type.Kind() != reflect.String {
				continue
			}
			v := reflect.ValueOf(&temp).Elem().Field(i)
			if v.String() != "" {
				v.SetString("*******")
			}
		}
	}

	return fmt.Sprintf("%+v", temp)
}

// allLogLevelsAsStrings returns all logrus levels as a list of strings
func allLogLevelsAsStrings() []string {
	var levels []string
	for _, level := range logrus.AllLevels {
		levels = append(levels, level.String())
	}
	return levels
}

// ParseFlags adds and parses flags from command line
func (cfg *Config) ParseFlags(args []string) error {
	app := kingpin.New("dns-tagger", "DNS Tagger Allows to change External DNS Records owner between clusters.\n\nNote that all flags may be replaced with env vars - `--flag` -> `EXTERNAL_DNS_DIALER_FLAG=1` or `--flag value` -> `EXTERNAL_DNS_DIALER_FLAG=value`")
	app.Version(Version)
	app.DefaultEnvars()

	// dns-tagger mode
	app.Flag("mode", "Determines the operation of the dns-tagger (default: owner, options: owner, resource)").Default(defaultConfig.Mode).EnumVar(&cfg.Mode, "owner", "resource")

	// Flags related to Kubernetes
	app.Flag("server", "The Kubernetes API server to connect to (default: auto-detect)").Default(defaultConfig.APIServerURL).StringVar(&cfg.APIServerURL)
	app.Flag("kubeconfig", "Retrieve target cluster configuration from a Kubernetes configuration file (default: auto-detect)").Default(defaultConfig.KubeConfig).StringVar(&cfg.KubeConfig)
	app.Flag("request-timeout", "Request timeout when calling Kubernetes APIs. 0s means no timeout").Default(defaultConfig.RequestTimeout.String()).DurationVar(&cfg.RequestTimeout)

	// Flags related to processing source
	app.Flag("source", "The resource types that are queried for endpoints; specify multiple times for multiple sources (required, options: ingress, istio-virtualservice").Required().PlaceHolder("source").EnumsVar(&cfg.Sources, "ingress", "istio-virtualservice")
	app.Flag("namespace", "Limit resources queried for endpoints to a specific namespace (default: all namespaces)").Default(defaultConfig.Namespace).StringVar(&cfg.Namespace)

	// Flags related to operations
	app.Flag("apply", "When enabled, executes dns changes (default: disabled)").BoolVar(&cfg.Apply)
	// TODO current-owner-id is optional in "resource" mode
	app.Flag("current-owner-id", "What owner id to set when records changing ownership").Required().StringVar(&cfg.CurrentOwnerID)
	// TODO previous-owner-id is optional in "resource" mode
	app.Flag("previous-owner-id", "What previous owner ids are allowed for migration").Required().PlaceHolder("previous-owner-id").StringsVar(&cfg.PreviousOwnerIDs)
	app.Flag("dns-zone", "What dns zone should be considered").Required().PlaceHolder("dns-zone").Default(cfg.DNSZones...).StringsVar(&cfg.DNSZones)

	// Miscellaneous flags
	app.Flag("log-format", "The format in which log messages are printed (default: text, options: text, json)").Default(defaultConfig.LogFormat).EnumVar(&cfg.LogFormat, "text", "json")
	app.Flag("log-level", "Set the level of logging. (default: info, options: panic, debug, info, warning, error, fatal)").Default(defaultConfig.LogLevel).EnumVar(&cfg.LogLevel, allLogLevelsAsStrings()...)

	_, err := app.Parse(args)
	if err != nil {
		return err
	}

	return nil
}
