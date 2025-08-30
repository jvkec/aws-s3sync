# s3sync - AWS S3 Simple Synchronization CLI Tool

A simple and powerful CLI tool to synchronize local directories with AWS S3 buckets. s3sync provides efficient file synchronization with manifest-based change detection, supporting both one-way sync (push/pull) and individual file operations.

## Features

- **Efficient sync**: Manifest-based change detection using SHA256 checksums
- **Bidirectional sync**: Push local files to S3 or pull remote files from S3
- **Conflict resolution**: Intelligent handling of concurrent changes
- **Easy setup**: Interactive setup wizard for AWS credentials
- **Bucket management**: Create and manage S3 buckets with security best practices
- **Dry-run mode**: Preview changes before executing
- **Progress tracking**: Visual feedback for file operations
- **Secure by default**: Encryption and versioning enabled on created buckets

### Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/jvkec/aws-s3sync.git
cd aws-s3sync

# Build the binary
go build -o s3sync ./cmd/s3sync

# Install globally (choose one method):

# Method 1: Move to /usr/local/bin (requires sudo on Unix-like systems)
sudo mv s3sync /usr/local/bin/

# Method 2: Add to your PATH by copying to ~/bin (Unix-like systems)
mkdir -p ~/bin
cp s3sync ~/bin/
# Then add this to your ~/.bashrc or ~/.zshrc:
# export PATH="$HOME/bin:$PATH"

# Method 3: Windows - copy to a directory in your PATH
# Example: copy s3sync.exe C:\Windows\System32\
```

### Prerequisites

- Go 1.24.1 or later
- AWS credentials (AWS CLI configured or IAM role)

## Quick Start

### 1. Setup Credentials

Run the interactive setup wizard:

```bash
s3sync setup
```

This will guide you through configuring:
- AWS region
- AWS credentials (profile or access keys)
- Default S3 bucket (optional)

### 2. Test Connection

Verify your AWS credentials work:

```bash
s3sync test-connection
```

### 3. Sync Files

Push local files to S3:

```bash
s3sync push ~/photos my-backup-bucket
```

Pull remote files from S3:

```bash
s3sync pull my-backup-bucket ~/downloads/photos
```

## Usage

### Basic Commands

```bash
# Setup wizard
s3sync setup

# Test AWS connection
s3sync test-connection

# List available buckets
s3sync list-buckets

# Create a new bucket
s3sync create-bucket my-new-bucket

# Scan local directory
s3sync scan ~/documents
```

### File Operations

```bash
# Upload single file
s3sync upload photo.jpg my-bucket
s3sync upload photo.jpg my-bucket photos/vacation.jpg

# Download single file
s3sync download my-bucket photos/vacation.jpg ./
s3sync download my-bucket photos/vacation.jpg ~/downloads/
```

### Directory Synchronization

```bash
# Push (upload) changes
s3sync push ~/documents my-backup-bucket
s3sync push ~/documents  # uses default bucket from config

# Pull (download) changes
s3sync pull my-backup-bucket ~/documents
s3sync pull my-backup-bucket ~/restored-files/

# Dry run to preview changes
s3sync push ~/documents my-bucket --dry-run
s3sync pull my-bucket ~/documents --dry-run
```

### Configuration Management

```bash
# Show current configuration
s3sync config show

# Configuration file location: ~/.s3sync/config.yaml
```

## How It Works

s3sync uses a manifest-based approach for efficient synchronization:

1. **Local manifest**: Scans local directory and creates checksums (SHA256)
2. **Remote manifest**: Lists S3 objects and uses ETags as checksums
3. **Last known state**: Tracks previous sync state for conflict detection
4. **Three-way diff**: Compares all three states to determine actions
5. **Conflict resolution**: Uses modification time when both sides changed

### Sync Logic

- **New local file** → upload to S3
- **New remote file** → download from S3
- **Local file modified** → upload to S3
- **Remote file modified** → download from S3
- **Both modified** → newer file wins (based on modification time)
- **File deleted locally** → respect local deletion (no download)
- **File deleted remotely** → re-upload local file

## Configuration

Configuration is stored in `~/.s3sync/config.yaml`:

```yaml
aws:
  region: us-east-1
  profile: my-profile  # or use access keys
sync:
  default_bucket: my-backup-bucket
  exclude_files:
    - ".ds_store"
    - "thumbs.db"
    - ".git/*"
  max_retries: 3
  chunk_size: 8388608  # 8MB
```

### AWS Credentials

s3sync supports multiple credential methods (in order of precedence):

1. **AWS profile**: `s3sync setup` → specify profile name
2. **Access keys**: `s3sync setup` → enter access key and secret
3. **Environment variables**: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
4. **IAM role**: Automatic for EC2 instances with attached roles
5. **AWS credentials file**: `~/.aws/credentials`

## Examples

### Backup Important Documents

```bash
# Initial setup
s3sync setup
s3sync create-bucket my-documents-backup-$(date +%s)

# Daily backup
s3sync push ~/documents my-documents-backup
```

### Sync Photos Between Devices

```bash
# Device 1: upload photos
s3sync push ~/photos shared-photos-bucket

# Device 2: download photos
s3sync pull shared-photos-bucket ~/photos
```

### Restore from Backup

```bash
# List available buckets
s3sync list-buckets

# Restore all files
s3sync pull my-backup-bucket ~/restored-files/

# Or restore specific files
s3sync download my-backup-bucket important-file.pdf ~/downloads/
```

## Security

s3sync follows AWS security best practices:

- **Encryption**: All created buckets use server-side encryption (AES256)
- **Versioning**: Enabled on all created buckets for data protection
- **Credentials**: Stored in secure configuration file with restricted permissions (0600)
- **Least privilege**: Only requires S3 permissions for specified buckets

### Required AWS Permissions

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket",
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:GetBucketLocation",
                "s3:CreateBucket",
                "s3:PutBucketVersioning",
                "s3:PutBucketEncryption"
            ],
            "Resource": [
                "arn:aws:s3:::your-bucket-name",
                "arn:aws:s3:::your-bucket-name/*"
            ]
        }
    ]
}
```

## Troubleshooting

### Common Issues

1. **Credentials not found**
   ```bash
   s3sync setup  # reconfigure credentials
   s3sync test-connection  # verify setup
   ```

2. **Bucket access denied**
   - Verify bucket exists and you have permissions
   - Check AWS region configuration

3. **Network timeout**
   - Check internet connection
   - Verify AWS service status

4. **Sync conflicts**
   - Use `--dry-run` to preview changes
   - Manually resolve conflicts if needed

### Debug Mode

Set environment variable for verbose logging:

```bash
export AWS_SDK_LOAD_CONFIG=1
export AWS_LOG_LEVEL=debug
s3sync push ~/documents my-bucket
```

## Development

### Project Structure

```
.
├── cmd/s3sync/          # CLI entry point
├── internal/
│   ├── aws/            # AWS S3 client and operations
│   ├── config/         # Configuration management
│   ├── fileutils/      # File scanning and utilities
│   └── sync/           # Sync logic and manifest handling
├── go.mod              # Go dependencies
└── README.md           # This file
```

### Building from Source

```bash
# Development build
go build -o s3sync ./cmd/s3sync

# Production build with optimizations
go build -ldflags="-s -w" -o s3sync ./cmd/s3sync

# Cross-platform builds
GOOS=windows GOARCH=amd64 go build -o s3sync.exe ./cmd/s3sync
GOOS=darwin GOARCH=arm64 go build -o s3sync-darwin-arm64 ./cmd/s3sync
GOOS=linux GOARCH=amd64 go build -o s3sync-linux-amd64 ./cmd/s3sync
```

### Testing

```bash
# Run tests
go test ./...

# Test with coverage
go test -cover ./...

# Test specific functionality
go test ./internal/sync -v
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Changelog

### v1.0.0 (current)

- Complete CLI interface with all core commands
- Manifest-based sync with conflict resolution
- AWS S3 integration with bucket management
- Interactive setup wizard
- Dry-run mode for safe testing
- Comprehensive error handling
- Security best practices (encryption, versioning)

## Roadmap

Future enhancements being considered:

- Bidirectional sync automation
- Exclude/include patterns (.gitignore style)
- Web interface for non-CLI users
- Bandwidth throttling
- Progress bars for large transfers
- Multiple destination support
- Notification system

---

For questions or support, please open an issue on GitHub.

