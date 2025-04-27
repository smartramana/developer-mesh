# S3 Storage Functionality

The MCP Server now supports storing context data in Amazon S3 or S3-compatible storage services. This feature enables efficient storage and retrieval of large context windows, with database references for indexing and querying.

## Overview

The S3 storage implementation provides the following benefits:

1. **Scalable Storage**: Efficiently store and retrieve large context data without database size limitations
2. **Cost Efficiency**: Reduce database storage costs by storing large context data in cheaper S3 storage
3. **Performance**: Optimize performance for large context operations using S3's multipart upload/download capabilities
4. **Flexibility**: Work with any S3-compatible storage service (AWS S3, MinIO, LocalStack, etc.)
5. **Security**: Support for server-side encryption options to protect context data

## Architecture

The S3 storage implementation follows a hybrid approach:

- **Context References**: Metadata and references to context data are stored in the database for efficient indexing and querying
- **Context Data**: The actual context content is stored in S3, with options for encryption and compression
- **Caching Layer**: Frequently accessed contexts are cached to reduce S3 access latency

This approach combines the strengths of both database and object storage systems, providing optimal performance for different use cases.

## Configuration

To enable and configure S3 storage, update your `config.yaml` file with the following settings:

```yaml
storage:
  # Storage provider type: "local" or "s3"
  type: "s3"
  
  # S3 Storage Configuration
  s3:
    region: "us-west-2"                          # AWS region
    bucket: "mcp-contexts"                        # S3 bucket name
    endpoint: "http://localhost:4566"             # Optional: custom endpoint for S3-compatible services
    force_path_style: true                        # Optional: required for some S3-compatible services
    server_side_encryption: "AES256"              # Optional: enable server-side encryption
    kms_key_id: ""                                # Optional: KMS key ID for aws:kms encryption
    upload_part_size: 5242880                     # Upload multipart size (5MB)
    download_part_size: 5242880                   # Download multipart size (5MB)
    concurrency: 5                                # Number of concurrent upload/download parts
    request_timeout: 30s                          # Timeout for S3 operations
  
  # Context Storage Configuration
  context_storage:
    # Provider: "database" or "s3"
    provider: "s3"                                # Use S3 for context storage
    s3_path_prefix: "contexts"                    # Prefix for S3 keys
```

Alternatively, you can use environment variables:

```bash
MCP_STORAGE_TYPE=s3
MCP_STORAGE_S3_REGION=us-west-2
MCP_STORAGE_S3_BUCKET=mcp-contexts
MCP_STORAGE_S3_ENDPOINT=http://localhost:4566
MCP_STORAGE_S3_FORCE_PATH_STYLE=true
MCP_STORAGE_S3_SERVER_SIDE_ENCRYPTION=AES256
MCP_STORAGE_S3_UPLOAD_PART_SIZE=5242880
MCP_STORAGE_S3_DOWNLOAD_PART_SIZE=5242880
MCP_STORAGE_S3_CONCURRENCY=5
MCP_STORAGE_S3_REQUEST_TIMEOUT=30s
MCP_STORAGE_CONTEXT_STORAGE_PROVIDER=s3
MCP_STORAGE_CONTEXT_STORAGE_S3_PATH_PREFIX=contexts
```

## Development and Testing

For local development and testing, the MCP Server includes integration with LocalStack, a lightweight AWS service emulator that runs in a Docker container. The Docker Compose configuration includes LocalStack with S3 support.

To run the MCP Server with LocalStack:

```bash
docker-compose up -d
```

This will start both the MCP Server and LocalStack, with S3 available at `http://localhost:4566`.

## Security Considerations

When configuring S3 storage for production use, consider the following security best practices:

1. **Server-Side Encryption**: Enable server-side encryption using either:
   - `AES256` (Amazon S3-managed keys)
   - `aws:kms` (AWS KMS-managed keys)
   
2. **IAM Policies**: Use IAM roles with the principle of least privilege when running in AWS environments

3. **Access Logging**: Enable S3 access logging for audit purposes

4. **Versioning**: Consider enabling bucket versioning to protect against accidental deletions

5. **Lifecycle Policies**: Implement lifecycle policies to manage context data retention

## Performance Tuning

To optimize performance when working with S3 storage, consider the following configuration options:

1. **Part Size**: Adjust `upload_part_size` and `download_part_size` based on your typical context sizes
   - Larger part sizes (up to 5GB) are more efficient for very large contexts
   - Smaller part sizes are more efficient for smaller contexts or lower bandwidth environments

2. **Concurrency**: Adjust the `concurrency` setting based on your system resources
   - Higher concurrency improves throughput for large files but increases memory usage
   - Lower concurrency reduces resource usage but may decrease throughput

3. **Timeout**: Set an appropriate `request_timeout` based on your network latency
   - Longer timeouts improve reliability in high-latency environments
   - Shorter timeouts improve responsiveness but may increase failures

## Client Usage

The client usage remains unchanged when using S3 storage. The MCP client API abstracts the storage implementation details, allowing seamless integration regardless of the underlying storage provider.

## Implementation Details

The S3 storage implementation uses the AWS SDK for Go v2, which provides modern, concurrent-safe interfaces for working with S3. Key components include:

1. **S3 Client**: Wrapper around the AWS SDK S3 client with optimized configuration
2. **Multipart Upload/Download**: Efficient transfer of large contexts using concurrent multipart operations
3. **Context ID to S3 Key Mapping**: Deterministic mapping between context IDs and S3 object keys
4. **S3 Error Handling**: Robust error handling with appropriate retry mechanisms
5. **Mock Implementation**: Test implementations for unit and integration testing

## Limitations

The current S3 implementation has the following limitations:

1. **No Cross-Region Replication**: Contexts are stored in a single region
2. **No Client-Side Encryption**: Only server-side encryption is supported
3. **Limited Compression**: Compression is not yet implemented for context data

These limitations will be addressed in future releases.

## Troubleshooting

Common issues and their solutions:

1. **Connection Refused**:
   - Check that the S3 endpoint is correctly configured
   - Verify network connectivity to the S3 endpoint

2. **Access Denied**:
   - Check AWS credentials and permissions
   - Verify bucket existence and access policies

3. **Slow Performance**:
   - Adjust multipart upload/download sizes
   - Increase concurrency
   - Check network latency to S3 endpoint

For detailed error information, check the MCP Server logs with the `debug` log level.