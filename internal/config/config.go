package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// config represents the application configuration
type Config struct {
	AWS     AWSConfig  `yaml:"aws"`
	Sync    SyncConfig `yaml:"sync"`
	Profile string     `yaml:"profile"`
}

// awsconfig contains aws-specific configuration
type AWSConfig struct {
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	SessionToken    string `yaml:"session_token,omitempty"`
	Profile         string `yaml:"profile,omitempty"`
}

// syncconfig contains sync-specific configuration
type SyncConfig struct {
	DefaultBucket string   `yaml:"default_bucket"`
	ExcludeFiles  []string `yaml:"exclude_files"`
	IncludeFiles  []string `yaml:"include_files"`
	MaxRetries    int      `yaml:"max_retries"`
	ChunkSize     int64    `yaml:"chunk_size"` // in bytes
}

// configmanager handles configuration operations
type ConfigManager struct {
	configPath string
}

// newconfigmanager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// fallback to current directory
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, ".s3sync")
	configPath := filepath.Join(configDir, "config.yaml")

	return &ConfigManager{
		configPath: configPath,
	}
}

// loadconfig loads configuration from file
func (cm *ConfigManager) LoadConfig() (*Config, error) {
	// create default config
	config := &Config{
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Sync: SyncConfig{
			ExcludeFiles: []string{".DS_Store", "Thumbs.db", ".git/*"},
			MaxRetries:   3,
			ChunkSize:    8 * 1024 * 1024, // 8mb chunks
		},
	}

	// if config file does not exist, return default config
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return config, nil
	}

	// read config file
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// parse yaml
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// saveconfig saves configuration to file
func (cm *ConfigManager) SaveConfig(config *Config) error {
	// ensure config directory exists
	configDir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// marshal to yaml
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// write to file
	if err := os.WriteFile(cm.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getconfigpath returns the path to the configuration file
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

// setupwizard runs an interactive setup wizard
func (cm *ConfigManager) SetupWizard() error {
	fmt.Println("🔧 s3sync setup wizard")
	fmt.Println("this will help you configure s3sync for first-time use")
	fmt.Println()

	config, err := cm.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load existing config: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)

	// aws region
	fmt.Printf("aws region [%s]: ", config.AWS.Region)
	region, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	region = strings.TrimSpace(region)
	if region != "" {
		config.AWS.Region = region
	}

	// aws profile or credentials
	fmt.Print("aws profile (leave empty to use access keys): ")
	profile, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	profile = strings.TrimSpace(profile)

	if profile != "" {
		config.AWS.Profile = profile
		config.Profile = profile
		fmt.Printf("✅ using aws profile: %s\n", profile)
	} else {
		// get access keys
		fmt.Print("aws access key id: ")
		accessKey, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		config.AWS.AccessKeyID = strings.TrimSpace(accessKey)

		fmt.Print("aws secret access key: ")
		secretKey, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		config.AWS.SecretAccessKey = strings.TrimSpace(secretKey)

		fmt.Print("aws session token (optional): ")
		sessionToken, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		sessionToken = strings.TrimSpace(sessionToken)
		if sessionToken != "" {
			config.AWS.SessionToken = sessionToken
		}
	}

	// default bucket
	fmt.Printf("default s3 bucket [%s]: ", config.Sync.DefaultBucket)
	bucket, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	bucket = strings.TrimSpace(bucket)
	if bucket != "" {
		config.Sync.DefaultBucket = bucket
	}

	// save configuration
	if err := cm.SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ configuration saved to: %s\n", cm.configPath)
	fmt.Println("you can now use s3sync to sync files to s3!")

	return nil
}

// validateconfig checks if the configuration is valid
func (config *Config) ValidateConfig() error {
	if config.AWS.Region == "" {
		return fmt.Errorf("aws region is required")
	}

	// check if either profile or access keys are provided
	hasProfile := config.AWS.Profile != ""
	hasAccessKeys := config.AWS.AccessKeyID != "" && config.AWS.SecretAccessKey != ""

	if !hasProfile && !hasAccessKeys {
		return fmt.Errorf("either aws profile or access keys must be provided")
	}

	return nil
}
