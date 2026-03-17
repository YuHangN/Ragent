# Ragent 开发日志

## 2026-03-17

1. 开始开发 Ragent 项目，今日主要目的是搭建项目骨架 + 配置 + 基础设施连接
2. 当前目录职责划分：
    - `cmd/`：存放项目的入口文件，包含 `main.go`，负责启动应用程序。
    - `internal/`：存放项目的内部实现代码，不对外暴露，包含核心业务逻辑和功能模块。
    - `pkg/`：存放可复用的公共库和工具函数，可以被其他项目引用。
    - `config/`：存放项目的配置文件，如 YAML、JSON 等格式的配置文件。
    - `infra/`: 存放项目的基础设施相关代码，如数据库连接、缓存连接等。
3. application.yaml 是项目的原始配置，需要 Config struct 作为接收器。Viper 读取配置文件后会将其解析为 Config struct 的实例， 
供应用程序使用。
4. viper 的 `SetEnvPrefix`, `SetEnvKeyReplacer` 相当于根据 yaml 里的规则进行替换，比如 `app.name` 会被替换成 `APP_NAME`，
然后在环境变量中查找 `APP_NAME` 的值来覆盖配置文件中的值。
5. AWS S3 配置