# cwsync

## intro

The **cwsync** is a go based application to export logs from AWS cloudwatch to centralized log storage systems. This tool helps consolidate logs in cloudwatch from various AWS services into a unified or distributed storage system, enabling efficient log management and analysis.

## purpose

In AWS cloud envs, services like AWS RDS, lambda, and load balancers automatically send logs to cloudwatch.  many other services used by differetn org, such as elasticsearch/opensearch for application logs and tools running in container orchestrators like kubernetes and nomad, do not.

The **cwsync** addresses this gap by exporting logs from cloudWatch to centralized/different log storage solutions like to a file (s) or kafka or elasticsearch , where they can be further processed or analyzed. This is particularly useful when you dont need to export AWS cloudwatch servces by relying solely on AWS services like Lambda and SQS using [elastic-serverless-forwarder](https://github.com/elastic/elastic-serverless-forwarder), instead opting to tail logs and output them to stdout, log files, or other destinations like kafka.

Key features of this include:

- **centralized log management**: consolidate logs from various sources into a single storage system.
- **offset Management**: store the offset of the last processed CloudWatch event in a local file (default), [Consul KV](https://developer.hashicorp.com/consul/docs/dynamic-app-config/kv), or AWS-native [DynamoDB](https://aws.amazon.com/dynamodb/), ensuring consistent log processing without loss or duplication.
- **customizable output**: tail logs and output them in a format that can be consumed by tools like [vector](https://vector.dev) for synchronization with [Elasticsearch](https://www.elastic.co/elasticsearch) or [kafka](https://kafka.apache.org/documentation/).

## features

- multi-service support for exporting logs from AWS and custom applications.
- support for various log storage destinations, including files, stdout, kafka and elasticsearch.
- offset management using a local file (default), Consul KV, or DynamoDB to ensure logs are processed consistently and to ensure delivery at-least-once.
- concurrent processing of multiple log streams for efficiency.
- configuration through a YAML file.

## architecture


1. **configuration loading**: settings are loaded from a YAML file, including AWS credentials, service details, and log destinations.
2. **offset store setup**: Offset store is selected (File default, Consul, or DynamoDB) and initialized once, then reused by all workers.
3. **log stream discovery**: relevant log streams are discovered based on the provided configuration.
4. **log taling**: Goroutines are initiated to fetch and process log events in real-time.
5. **offset management**: processed logs are saved back to the selected offset store to maintain state across restarts.
6. **log output**: logs are output to specified destinations for further consumption or analysis.

## prerequisites

before deploying and running the **cwsync**, ensure the following:

- **AWS Credentials**: AWS creds with appropriate permissions to access cloudwatch Logs.
- **consul (optional)**: a running Consul instance if using Consul for offset management.
- **dynamodb (optional)**: a DynamoDB table if using DynamoDB for offset management.
- **log storage and management**: Access to desired log storage systems like Elasticsearch, Kafka, or file systems.

## configuration

The application is configured using a YAML file (`config.yaml`). Below are sample configurations for the File (default), Consul, and DynamoDB offset stores. You can also set env vars to override config (see 12-factor section below).

```yaml
# Using File as offset store (default)
offset_store: "file"  # default for all services; per-service override possible
file:
  path: "/var/lib/cwsync/offsetstore.json"  # optional; defaults to executable directory

aws_region: "us-east-1"
aws_profile: "default"
aws_role_arn: "arn:aws:iam::123456789012:role/role"
aws_access_key: "YOUR_ACCESS_KEY"
aws_secret_key: "YOUR_SECRET_KEY"

services:
  - name: "my-service"
    consul_kv_path: "log_offsets/my-service"  # used as key prefix across all stores
    log_configs:
      - log_group_name: "/aws/lambda/my-function"
        log_stream_prefix: "function-name"
    destination:
      type: "file"
      file_path: "/var/logs/my-service"
      file_name: "logs.txt"

---

# Using Consul as offset store
consul:
  address: "http://localhost:8500"
  token: "your-consul-token"

offset_store: "consul"
services:
  - name: "my-service"
    consul_kv_path: "log_offsets/my-service"
    log_configs:
      - log_group_name: "/aws/lambda/my-function"
        log_stream_prefix: "function-name"
    destination:
      type: "file"
      file_path: "/var/logs/my-service"
      file_name: "logs.txt"

---

# Using DynamoDB as offset store (AWS-native)
dynamodb:
  table_name: "cwsync_offsets"

aws_region: "us-east-1"
aws_profile: "default"

services:
  - name: "my-service"
    offset_store: "dynamodb"   # choose DynamoDB for offsets
    consul_kv_path: "log_offsets/my-service"  # reused as the DynamoDB item key prefix
    log_configs:
      - log_group_name: "/aws/lambda/my-function"
        log_stream_prefix: "function-name"
    destination:
      type: "file"
      file_path: "/var/logs/my-service"
      file_name: "logs.txt"
```
### configuration parameters
- consul configuration:
  - consul.address: The http address of your consul server.
  - consul.token: The access token for consul.
- file configuration:
  - file.path: Path to the JSON file that stores offsets. Defaults to `<executable-dir>/offsetstore.json`.
- DynamoDB configuration:
  - dynamodb.table_name: Name of the DynamoDB table to store offsets. The table must exist with a string partition key named `key`.
- AWS Configuration:
  - aws_region: AWS region for CloudWatch logs.
  - aws_profile: (optional) AWS CLI profile for credentials.
  - aws_role_arn: (optional) ARN of the AWS IAM role to assume.
  - aws_access_key & aws_secret_key: (optional) Static AWS credentials.
- Services Configuration:
  - services: list of services to monitor and export logs for.
  - name: identifier for the service.
  - offset_store: default at root; per service you can set `file`, `consul` or `dynamodb`.
  - consul_kv_path: path prefix for saving offsets. For Consul, this is the KV path. For DynamoDB, this value is reused as the item key prefix; the final key is `<consul_kv_path>/<log_stream_name>`.
  - log_configs: list of log groups and streams to monitor.
  - destination: defines where to output logs (e.g., file, stdout).


## usage

Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/copernicium-112/cwsync.git
   cd cwsync
   ```

2. Build the application:
   ```bash
   go build -o cwsync main.go
   ```

### configuration setup
1. create configuration file:
    Create a config.yaml file in the application directory with your specific settings.
2. verify access:
    ensure the application can access AWS services and Consul with the provided credentials.

## project structure

- `internal/logger`: shared loggers for consistent output levels.
- `internal/offsetstore`: offset persistence interface and implementations (`file`, `consul`, `dynamodb`).
- `internal/tailer`: CloudWatch Logs listing and tailing logic using the pluggable offset store.
- `main.go`: wiring for config, AWS session, store initialization, and worker startup.

## DynamoDB offset store

When selecting `offset_store: dynamodb`, cwsync stores offsets in a DynamoDB table for an AWS-native experience (useful when shipping to Elastic or Kafka outside AWS while keeping state in AWS).

- Table schema:
  - Partition key: `key` (String)
  - Attribute: `last_timestamp` (Number, milliseconds since epoch)
- Item key format: `<consul_kv_path>/<log_stream_name>` (reuses existing path as a simple, unique key)
- Minimal IAM permissions:
  - `dynamodb:GetItem` on the table
  - `dynamodb:PutItem` on the table

Example AWS CLI to create a table:

```bash
aws dynamodb create-table \
  --table-name cwsync_offsets \
  --attribute-definitions AttributeName=key,AttributeType=S \
  --key-schema AttributeName=key,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

Fallback behavior: if no offset item exists, cwsync starts from `now - offset_fallback_duration`.

## File offset store (default)

The default store persists offsets to a single JSON file.

- File path: `file.path` in config, or defaults to the executable directory (`offsetstore.json`).
- Format: a JSON object mapping `<consul_kv_path>/<log_stream_name>` to last timestamp in milliseconds.
- Startup behavior: if no file or entry exists, cwsync starts from `now - offset_fallback_duration` and then persists offsets as logs are processed.
- Concurrency: safe for concurrent goroutines in a single process. For multi-process writes to the same file, use Consul/DynamoDB or add a file lock.

##  configuration

All critical settings can be supplied via environment variables to deployments. These override values from `config.yaml` when present:

- `CWSYNC_OFFSET_STORE`: `file`, `consul`, or `dynamodb` (default is `file`)
- `CWSYNC_OFFSET_FILE`: path to the JSON offset file
- `CWSYNC_AWS_REGION`: AWS region
- `CWSYNC_AWS_PROFILE`: AWS profile
- `CWSYNC_AWS_ROLE_ARN`: Assume role ARN
- `CWSYNC_AWS_ACCESS_KEY`, `CWSYNC_AWS_SECRET_KEY`: static credentials
- `CWSYNC_CONSUL_ADDR`, `CWSYNC_CONSUL_TOKEN`: Consul address/token
- `CWSYNC_DYNAMODB_TABLE`: DynamoDB table name for offsets
- `CWSYNC_OFFSET_FALLBACK_DURATION`: e.g., `1h`, `15m`

Tip: Keep the YAML as a baseline and drive per-environment differences via env vars.
