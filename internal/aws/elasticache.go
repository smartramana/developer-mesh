package aws

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticachev2"
	"github.com/aws/aws-sdk-go-v2/service/elasticachev2/types"
)

// ElastiCacheConfig holds configuration for ElastiCache
type ElastiCacheConfig struct {
	AuthConfig          AuthConfig `mapstructure:"auth"`
	PrimaryEndpoint     string     `mapstructure:"primary_endpoint"`
	Port                int        `mapstructure:"port"`
	Username            string     `mapstructure:"username"` // For Redis auth
	Password            string     `mapstructure:"password"` // For Redis auth
	UseIAMAuth          bool       `mapstructure:"use_iam_auth"`
	ClusterMode         bool       `mapstructure:"cluster_mode"`
	ReaderEndpoint      string     `mapstructure:"reader_endpoint"` // Used for cluster mode
	CacheNodes          []string   `mapstructure:"cache_nodes"` // List of nodes for cluster mode
	ClusterDiscovery    bool       `mapstructure:"cluster_discovery"` // Use API to discover nodes
	ClusterName         string     `mapstructure:"cluster_name"`
	UseTLS              bool       `mapstructure:"use_tls"`
	InsecureSkipVerify  bool       `mapstructure:"insecure_skip_verify"`
	MaxRetries          int        `mapstructure:"max_retries"`
	MinIdleConnections  int        `mapstructure:"min_idle_connections"`
	PoolSize            int        `mapstructure:"pool_size"`
	DialTimeout         int        `mapstructure:"dial_timeout"`
	ReadTimeout         int        `mapstructure:"read_timeout"`
	WriteTimeout        int        `mapstructure:"write_timeout"`
	PoolTimeout         int        `mapstructure:"pool_timeout"`
	TokenExpiration     int        `mapstructure:"token_expiration"`
}

// ElastiCacheClient is a client for AWS ElastiCache
type ElastiCacheClient struct {
	client   *elasticachev2.Client
	legacyClient *elasticache.Client
	config   ElastiCacheConfig
}

// NewElastiCacheClient creates a new ElastiCache client
func NewElastiCacheClient(ctx context.Context, cfg ElastiCacheConfig) (*ElastiCacheClient, error) {
	// Get AWS configuration
	awsCfg, err := GetAWSConfig(ctx, cfg.AuthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create ElastiCache clients - we need both v1 and v2 for different operations
	client := elasticachev2.NewFromConfig(awsCfg)
	legacyClient := elasticache.NewFromConfig(awsCfg)

	return &ElastiCacheClient{
		client:   client,
		legacyClient: legacyClient,
		config:   cfg,
	}, nil
}

// GetAuthToken generates a temporary IAM auth token for ElastiCache
func (c *ElastiCacheClient) GetAuthToken(ctx context.Context) (string, error) {
	input := &elasticachev2.CreateUserAccessStringInput{
		User: aws.String(c.config.Username),
	}
	
	if c.config.TokenExpiration > 0 {
		duration := int32(c.config.TokenExpiration)
		input.TimeToLive = &duration
	}
	
	response, err := c.client.CreateUserAccessString(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to generate ElastiCache auth token: %w", err)
	}
	
	if response.AccessString == nil {
		return "", fmt.Errorf("received nil access string from ElastiCache")
	}
	
	return *response.AccessString, nil
}

// DiscoverClusterNodes discovers all nodes in a Redis cluster
func (c *ElastiCacheClient) DiscoverClusterNodes(ctx context.Context) ([]string, error) {
	if c.config.ClusterName == "" {
		return nil, fmt.Errorf("cluster name is required for node discovery")
	}

	// Using legacy API (v1) since v2 doesn't have the describe functionality yet
	input := &elasticache.DescribeCacheClustersInput{
		CacheClusterId:    aws.String(c.config.ClusterName),
		ShowCacheNodeInfo: aws.Bool(true),
	}

	result, err := c.legacyClient.DescribeCacheClusters(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cache cluster: %w", err)
	}

	if len(result.CacheClusters) == 0 {
		return nil, fmt.Errorf("no cache clusters found with name: %s", c.config.ClusterName)
	}

	var nodes []string
	for _, cluster := range result.CacheClusters {
		for _, node := range cluster.CacheNodes {
			if node.Endpoint != nil && node.Endpoint.Address != nil && node.Endpoint.Port != nil {
				nodeAddr := fmt.Sprintf("%s:%d", *node.Endpoint.Address, *node.Endpoint.Port)
				nodes = append(nodes, nodeAddr)
			}
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found in cache cluster: %s", c.config.ClusterName)
	}

	return nodes, nil
}

// BuildRedisOptions builds options for connecting to Redis
func (c *ElastiCacheClient) BuildRedisOptions(ctx context.Context) (map[string]interface{}, error) {
	options := make(map[string]interface{})
	
	// Determine if we're using cluster mode
	if c.config.ClusterMode {
		// In cluster mode, we need a list of nodes
		var nodes []string
		
		// If cluster discovery is enabled, use the AWS API to discover nodes
		if c.config.ClusterDiscovery {
			discoveredNodes, err := c.DiscoverClusterNodes(ctx)
			if err != nil {
				return nil, err
			}
			nodes = discoveredNodes
		} else if len(c.config.CacheNodes) > 0 {
			// Use explicitly configured nodes
			nodes = c.config.CacheNodes
		} else if c.config.PrimaryEndpoint != "" {
			// Use primary endpoint as fallback
			primaryAddr := fmt.Sprintf("%s:%d", c.config.PrimaryEndpoint, c.config.Port)
			nodes = []string{primaryAddr}
			
			// If we have a reader endpoint, add it too
			if c.config.ReaderEndpoint != "" {
				readerAddr := fmt.Sprintf("%s:%d", c.config.ReaderEndpoint, c.config.Port)
				nodes = append(nodes, readerAddr)
			}
		} else {
			return nil, fmt.Errorf("no nodes available for cluster mode - either configure cache_nodes, enable cluster_discovery, or provide primary_endpoint")
		}
		
		options["clusterMode"] = true
		options["addrs"] = nodes
	} else {
		// Standard mode - just use the primary endpoint
		if c.config.PrimaryEndpoint == "" {
			return nil, fmt.Errorf("primary endpoint is required for non-cluster mode")
		}
		
		options["addr"] = fmt.Sprintf("%s:%d", c.config.PrimaryEndpoint, c.config.Port)
		options["clusterMode"] = false
	}
	
	// Authentication
	if c.config.UseIAMAuth && IsIRSAEnabled() {
		token, err := c.GetAuthToken(ctx)
		if err != nil {
			return nil, err
		}
		options["username"] = c.config.Username
		options["password"] = token
	} else if c.config.Password != "" {
		// Fall back to standard password auth
		if c.config.Username != "" {
			options["username"] = c.config.Username
		}
		options["password"] = c.config.Password
	}
	
	// TLS configuration
	if c.config.UseTLS {
		options["tls"] = &tls.Config{
			InsecureSkipVerify: c.config.InsecureSkipVerify,
		}
	}
	
	// Connection pool settings
	options["poolSize"] = c.config.PoolSize
	options["minIdleConns"] = c.config.MinIdleConnections
	options["maxRetries"] = c.config.MaxRetries
	options["dialTimeout"] = time.Duration(c.config.DialTimeout) * time.Second
	options["readTimeout"] = time.Duration(c.config.ReadTimeout) * time.Second
	options["writeTimeout"] = time.Duration(c.config.WriteTimeout) * time.Second
	options["poolTimeout"] = time.Duration(c.config.PoolTimeout) * time.Second
	
	return options, nil
}

// GetClusters gets information about ElastiCache Redis clusters
func (c *ElastiCacheClient) GetClusters(ctx context.Context) ([]types.CacheSummary, error) {
	input := &elasticachev2.GetClustersInput{
		MaxResults: aws.Int32(20),
	}
	
	result, err := c.client.GetClusters(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusters: %w", err)
	}
	
	return result.Clusters, nil
}

// ParseRedisURL parses a Redis URL into separate components
func ParseRedisURL(url string) (host string, port int, err error) {
	// Remove protocol prefix if present
	url = strings.TrimPrefix(url, "redis://")
	url = strings.TrimPrefix(url, "rediss://")
	
	// Split host and port
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid Redis URL format: %s", url)
	}
	
	host = parts[0]
	fmt.Sscanf(parts[1], "%d", &port)
	
	if port == 0 {
		port = 6379 // Default Redis port
	}
	
	return host, port, nil
}
