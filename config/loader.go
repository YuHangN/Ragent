package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Load 负责读取 application.yaml，并把内容解析到 Config 结构体里并返回.
func Load() *Config {
	// 创建一个独立的 viper 实例。
	v := viper.New()

	v.SetConfigName("application")
	v.SetConfigType("yaml")
	// 表示当前工作目录下寻找配置文件。
	v.AddConfigPath(".")
	// 也可以在 ./config/ 目录下寻找配置文件，优先级低于当前目录。
	v.AddConfigPath("./config")

	// 开启环境变量覆盖配置的能力。
	// 例如：RAGENT_DB_DSN=xxxx 可以覆盖 yaml 里的 db.dsn
	// 相当于按照 yaml 的规则读取的时候，会先尝试替换成对应的环境变量名字查找，如果找到了就用环境变量的值，否则才用 yaml 里的值。
	v.SetEnvPrefix("RAGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("解析配置失败: %v", err)
	}

	return &cfg
}
