package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jvkec/aws-s3sync/internal/aws"
	"github.com/jvkec/aws-s3sync/internal/config"
	"github.com/jvkec/aws-s3sync/internal/fileutils"
	"github.com/jvkec/aws-s3sync/internal/sync"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "s3sync",
	Short: "s3sync is a simple tool to sync files to and from s3",
	Long: `a simple and flexible cli tool to synchronize local directories with an s3 bucket.
it supports pushing local changes to s3 and pulling remote changes from s3.`,
	Run: func(cmd *cobra.Command, args []string) {
		// default action when no subcommand is given
		fmt.Println("welcome to s3sync. use 's3sync --help' to see available commands.")
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "run interactive setup wizard",
	Long:  `runs an interactive setup wizard to configure aws credentials and default settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		configManager := config.NewConfigManager()
		if err := configManager.SetupWizard(); err != nil {
			fmt.Printf("error during setup: %v\n", err)
			os.Exit(1)
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "manage configuration",
	Long:  `manage s3sync configuration settings.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "show current configuration",
	Long:  `displays the current configuration settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("configuration file: %s\n", configManager.GetConfigPath())
		fmt.Printf("aws region: %s\n", cfg.AWS.Region)
		if cfg.AWS.Profile != "" {
			fmt.Printf("aws profile: %s\n", cfg.AWS.Profile)
		} else {
			fmt.Printf("aws access key: %s\n", maskCredential(cfg.AWS.AccessKeyID))
		}
		fmt.Printf("default bucket: %s\n", cfg.Sync.DefaultBucket)
		fmt.Printf("max retries: %d\n", cfg.Sync.MaxRetries)
		fmt.Printf("chunk size: %d mb\n", cfg.Sync.ChunkSize/(1024*1024))
	},
}

var testConnectionCmd = &cobra.Command{
	Use:   "test-connection",
	Short: "test aws credentials and connection",
	Long:  `verifies that aws credentials work and can connect to s3.`,
	Run: func(cmd *cobra.Command, args []string) {
		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		if err := cfg.ValidateConfig(); err != nil {
			fmt.Printf("invalid configuration: %v\n", err)
			fmt.Println("run 's3sync setup' to configure credentials")
			os.Exit(1)
		}

		client, err := aws.NewClient(cfg)
		if err != nil {
			fmt.Printf("error creating aws client: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		if err := client.TestConnection(ctx); err != nil {
			fmt.Printf("connection failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ aws connection successful!")
	},
}

var listBucketsCmd = &cobra.Command{
	Use:   "list-buckets",
	Short: "list accessible s3 buckets",
	Long:  `lists all s3 buckets accessible with current credentials.`,
	Run: func(cmd *cobra.Command, args []string) {
		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		client, err := aws.NewClient(cfg)
		if err != nil {
			fmt.Printf("error creating aws client: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		buckets, err := client.ListBuckets(ctx)
		if err != nil {
			fmt.Printf("error listing buckets: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("accessible s3 buckets (%d):\n", len(buckets))
		for _, bucket := range buckets {
			fmt.Printf("  - %s\n", bucket)
		}
	},
}

var createBucketCmd = &cobra.Command{
	Use:   "create-bucket [bucket-name]",
	Short: "create a new s3 bucket",
	Long:  `creates a new s3 bucket with appropriate security settings.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bucketName := args[0]

		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		client, err := aws.NewClient(cfg)
		if err != nil {
			fmt.Printf("error creating aws client: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		fmt.Printf("creating bucket: %s\n", bucketName)
		if err := client.CreateBucket(ctx, bucketName); err != nil {
			fmt.Printf("error creating bucket: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ bucket created successfully: %s\n", bucketName)
	},
}

var uploadCmd = &cobra.Command{
	Use:   "upload [local-file] [bucket-name] [s3-key]",
	Short: "upload a single file to s3",
	Long:  `uploads a single file to the specified s3 bucket.`,
	Args:  cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		localFile := args[0]
		bucketName := args[1]
		s3Key := ""

		if len(args) == 3 {
			s3Key = args[2]
		} else {
			s3Key = filepath.Base(localFile)
		}

		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		client, err := aws.NewClient(cfg)
		if err != nil {
			fmt.Printf("error creating aws client: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		fmt.Printf("uploading %s to s3://%s/%s\n", localFile, bucketName, s3Key)
		if err := client.UploadFile(ctx, localFile, bucketName, s3Key); err != nil {
			fmt.Printf("error uploading file: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ file uploaded successfully!")
	},
}

var downloadCmd = &cobra.Command{
	Use:   "download [bucket-name] [s3-key] [local-path]",
	Short: "download a single file from s3",
	Long:  `downloads a single file from the specified s3 bucket.`,
	Args:  cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		bucketName := args[0]
		s3Key := args[1]
		localPath := "./"

		if len(args) == 3 {
			localPath = args[2]
		}

		// if local path is a directory, use the s3 key filename
		if stat, err := os.Stat(localPath); err == nil && stat.IsDir() {
			localPath = filepath.Join(localPath, filepath.Base(s3Key))
		} else if localPath == "./" {
			localPath = filepath.Base(s3Key)
		}

		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		client, err := aws.NewClient(cfg)
		if err != nil {
			fmt.Printf("error creating aws client: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		fmt.Printf("downloading s3://%s/%s to %s\n", bucketName, s3Key, localPath)
		if err := client.DownloadFile(ctx, bucketName, s3Key, localPath); err != nil {
			fmt.Printf("error downloading file: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ file downloaded successfully!")
	},
}

var pushCmd = &cobra.Command{
	Use:   "push [local-path] [bucket-name]",
	Short: "push local files to s3",
	Long:  `pushes files from a local directory to an s3 bucket.`,
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		localPath := args[0]
		bucketName := ""

		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		if len(args) == 2 {
			bucketName = args[1]
		} else if cfg.Sync.DefaultBucket != "" {
			bucketName = cfg.Sync.DefaultBucket
		} else {
			fmt.Println("error: bucket name required (no default bucket configured)")
			fmt.Println("usage: s3sync push [local-path] [bucket-name]")
			fmt.Println("or run 's3sync setup' to configure a default bucket")
			os.Exit(1)
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if err := performPush(localPath, bucketName, cfg, dryRun); err != nil {
			fmt.Printf("error during push: %v\n", err)
			os.Exit(1)
		}
	},
}

var pullCmd = &cobra.Command{
	Use:   "pull [bucket-name] [local-path]",
	Short: "pull remote files from s3",
	Long:  `pulls files from an s3 bucket to a local directory.`,
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		bucketName := args[0]
		localPath := "./"

		if len(args) == 2 {
			localPath = args[1]
		}

		configManager := config.NewConfigManager()
		cfg, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if err := performPull(bucketName, localPath, cfg, dryRun); err != nil {
			fmt.Printf("error during pull: %v\n", err)
			os.Exit(1)
		}
	},
}

var scanCmd = &cobra.Command{
	Use:   "scan [local-path]",
	Short: "scan a local directory and show what would be synced",
	Long:  `scans a local directory and displays a list of files that would be synchronized.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("error: missing local_path argument")
			os.Exit(1)
		}
		localPath := args[0]
		files, err := fileutils.ScanDirectoryWithInfo(localPath)
		if err != nil {
			fmt.Printf("error scanning directory: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("files in %s (%d files):\n", localPath, len(files))
		for _, file := range files {
			fmt.Printf("  %s (%d bytes, %s)\n", file.RelativePath, file.Size, file.ModTime.Format("2006-01-02 15:04:05"))
		}
	},
}

// helper functions

func maskCredential(credential string) string {
	if credential == "" {
		return "(not set)"
	}
	if len(credential) <= 4 {
		return strings.Repeat("*", len(credential))
	}
	return credential[:4] + strings.Repeat("*", len(credential)-4)
}

func performPush(localPath, bucketName string, cfg *config.Config, dryRun bool) error {
	// create aws client
	client, err := aws.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create aws client: %w", err)
	}

	ctx := context.Background()

	// check if bucket exists
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("error checking bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %s does not exist or is not accessible", bucketName)
	}

	// create manifest manager
	manifestManager := sync.NewManifestManager(localPath)

	// load last known manifest
	lastManifest, err := manifestManager.LoadManifest()
	if err != nil {
		return fmt.Errorf("error loading manifest: %w", err)
	}

	// build current local manifest
	localManifest, err := manifestManager.BuildLocalManifest(localPath)
	if err != nil {
		return fmt.Errorf("error scanning local directory: %w", err)
	}
	localManifest.Bucket = bucketName

	// get remote manifest by listing s3 objects
	remoteFiles, err := client.ListObjects(ctx, bucketName, "")
	if err != nil {
		return fmt.Errorf("error listing remote objects: %w", err)
	}

	remoteManifest := &sync.Manifest{
		Files:  make(map[string]fileutils.FileInfo),
		Bucket: bucketName,
	}
	for _, file := range remoteFiles {
		remoteManifest.Files[file.RelativePath] = file
	}

	// compute sync actions
	actions := sync.ComputeSyncActions(localManifest, remoteManifest, lastManifest)

	// display actions
	uploadCount := 0
	skipCount := 0
	for _, action := range actions {
		switch action.Operation {
		case sync.SyncOpUpload:
			uploadCount++
			if dryRun {
				fmt.Printf("[dry-run] would upload: %s (%s)\n", action.RelativePath, action.Reason)
			}
		case sync.SyncOpSkip:
			skipCount++
		}
	}

	fmt.Printf("📦 push summary: %d files to upload, %d files to skip\n", uploadCount, skipCount)

	if dryRun {
		fmt.Println("dry-run mode: no files were actually uploaded")
		return nil
	}

	if uploadCount == 0 {
		fmt.Println("✅ everything up to date!")
		return nil
	}

	// perform uploads
	for _, action := range actions {
		if action.Operation == sync.SyncOpUpload {
			fmt.Printf("⬆️  uploading %s...\n", action.RelativePath)
			localFilePath := filepath.Join(localPath, action.RelativePath)
			if err := client.UploadFile(ctx, localFilePath, bucketName, action.RelativePath); err != nil {
				return fmt.Errorf("error uploading %s: %w", action.RelativePath, err)
			}
		}
	}

	// save updated manifest
	if err := manifestManager.SaveManifest(localManifest); err != nil {
		return fmt.Errorf("error saving manifest: %w", err)
	}

	fmt.Printf("✅ synced %d files to s3 bucket %s\n", uploadCount, bucketName)
	return nil
}

func performPull(bucketName, localPath string, cfg *config.Config, dryRun bool) error {
	// create aws client
	client, err := aws.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create aws client: %w", err)
	}

	ctx := context.Background()

	// check if bucket exists
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("error checking bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %s does not exist or is not accessible", bucketName)
	}

	// ensure local directory exists
	if err := fileutils.CreateDirIfNotExists(localPath); err != nil {
		return fmt.Errorf("error creating local directory: %w", err)
	}

	// create manifest manager
	manifestManager := sync.NewManifestManager(localPath)

	// load last known manifest
	lastManifest, err := manifestManager.LoadManifest()
	if err != nil {
		return fmt.Errorf("error loading manifest: %w", err)
	}

	// build current local manifest
	localManifest, err := manifestManager.BuildLocalManifest(localPath)
	if err != nil {
		return fmt.Errorf("error scanning local directory: %w", err)
	}

	// get remote manifest by listing s3 objects
	remoteFiles, err := client.ListObjects(ctx, bucketName, "")
	if err != nil {
		return fmt.Errorf("error listing remote objects: %w", err)
	}

	remoteManifest := &sync.Manifest{
		Files:  make(map[string]fileutils.FileInfo),
		Bucket: bucketName,
	}
	for _, file := range remoteFiles {
		remoteManifest.Files[file.RelativePath] = file
	}

	// compute sync actions
	actions := sync.ComputeSyncActions(localManifest, remoteManifest, lastManifest)

	// display actions
	downloadCount := 0
	skipCount := 0
	for _, action := range actions {
		switch action.Operation {
		case sync.SyncOpDownload:
			downloadCount++
			if dryRun {
				fmt.Printf("[dry-run] would download: %s (%s)\n", action.RelativePath, action.Reason)
			}
		case sync.SyncOpSkip:
			skipCount++
		}
	}

	fmt.Printf("📦 pull summary: %d files to download, %d files to skip\n", downloadCount, skipCount)

	if dryRun {
		fmt.Println("dry-run mode: no files were actually downloaded")
		return nil
	}

	if downloadCount == 0 {
		fmt.Println("✅ everything up to date!")
		return nil
	}

	// perform downloads
	for _, action := range actions {
		if action.Operation == sync.SyncOpDownload {
			fmt.Printf("⬇️  downloading %s...\n", action.RelativePath)
			localFilePath := filepath.Join(localPath, action.RelativePath)
			if err := client.DownloadFile(ctx, bucketName, action.RelativePath, localFilePath); err != nil {
				return fmt.Errorf("error downloading %s: %w", action.RelativePath, err)
			}
		}
	}

	// save updated manifest
	remoteManifest.Bucket = bucketName
	if err := manifestManager.SaveManifest(remoteManifest); err != nil {
		return fmt.Errorf("error saving manifest: %w", err)
	}

	fmt.Printf("✅ synced %d files from s3 bucket %s\n", downloadCount, bucketName)
	return nil
}

func init() {
	// add dry-run flag to push and pull commands
	pushCmd.Flags().Bool("dry-run", false, "show what would be done without actually doing it")
	pullCmd.Flags().Bool("dry-run", false, "show what would be done without actually doing it")

	// add subcommands to config
	configCmd.AddCommand(configShowCmd)

	// add all commands to root
	rootCmd.AddCommand(
		setupCmd,
		configCmd,
		testConnectionCmd,
		listBucketsCmd,
		createBucketCmd,
		uploadCmd,
		downloadCmd,
		pushCmd,
		pullCmd,
		scanCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
