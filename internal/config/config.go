package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

var AppVersion = "dev"

const (
	DefaultNewsURL = "https://www.cherkasyoblenergo.com/api/v1/posts/category/news?lang=uk&page=0&size=20"
	legacyNewsPath = "obl-main-controller/api/news2"
)

type Config struct {
	DBName     string `mapstructure:"DB_NAME"`
	ServerPort string `mapstructure:"SERVER_PORT"`
	LogLevel   string `mapstructure:"LOG_LEVEL"`
	NewsURL    string `mapstructure:"NEWS_URL"`

	RateLimitPerMinute int `mapstructure:"RATE_LIMIT_PER_MINUTE"`

	APIKey string `mapstructure:"API_KEY"`

	ProxyMode string `mapstructure:"PROXY_MODE"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")

	viper.SetDefault("RATE_LIMIT_PER_MINUTE", 60)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("NEWS_URL", DefaultNewsURL)
	viper.SetDefault("PROXY_MODE", "none")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return
	}

	if strings.Contains(config.NewsURL, legacyNewsPath) {
		log.Printf("NEWS_URL points to the retired legacy API (%s), using %s instead", config.NewsURL, DefaultNewsURL)
		config.NewsURL = DefaultNewsURL
	}
	return
}
