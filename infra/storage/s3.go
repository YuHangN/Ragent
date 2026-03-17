package storage

import (
	"context"
	"fmt"
	"log"

	"github.com/YuHangN/ragent-go/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// NewS3Client 负责初始化 S3 客户端。
// TODO: 需要理解代码，特别是 AWS 配置部分
func NewS3Client(cfg *config.RustFSConfig) *s3.Client {
	// 根据配置决定使用 http 还是 https。
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}

	// 拼出完整 endpoint
	endpoint := fmt.Sprintf("%s://%s", scheme, cfg.Endpoint)

	// 手动构造 AWS 配置。
	awsCfg := aws.Config{
		Region: "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					HostnameImmutable: true,
				}, nil
			},
		),
	}

	// 基于上面的配置创建 S3 Client。
	// UsePathStyle = true 对很多本地 S3 兼容存储很重要。
	// 否则它可能会去拼 bucket.xxx.com 这种 virtual-host 风格地址。
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// 启动阶段尝试探测 bucket 是否可访问。
	// 如果 bucket 还没创建，这里不直接 fatal，而是打一个警告。
	_, err := client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		log.Printf("警告: S3/RustFS bucket '%s' 访问失败: %v（可能还未创建）", cfg.Bucket, err)
	} else {
		fmt.Println("✓ S3/RustFS 连接成功")
	}

	return client
}
