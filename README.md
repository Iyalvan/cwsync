# cwsync

## intro

The **cwsync** is a go based application to export logs from AWS cloudwatch to centralized log storage systems. This tool helps consolidate logs in cloudwatch from various AWS services into a unified or distributed storage system, enabling efficient log management and analysis.

## purpose

In AWS cloud envs, services like AWS RDS, lambda, and load balancers automatically send logs to cloudwatch.  many other services used by differetn org, such as elasticsearch/opensearch for application logs and tools running in container orchestrators like kubernetes and nomad, do not.

The **cwsync** addresses this gap by tailing logs from cloudWatch and writing each event to stdout as one json line, where a shipper such as [vector](https://vector.dev) can forward it to centralized log storage like files, kafka or elasticsearch for further processing or analysis. This is particularly useful when you dont need to export AWS cloudwatch servces by relying solely on AWS services like Lambda and SQS using [elastic-serverless-forwarder](https://github.com/elastic/elastic-serverless-forwarder), instead opting to tail logs and output them to stdout.

Key features of this include:

- **centralized log management**: consolidate logs from various sources into a single storage system.
- **offset Management**: utilize [consul KV](https://developer.hashicorp.com/consul/docs/dynamic-app-config/kv) to store the offset of the cloudwatch log stream, ensuring consistent log processing without loss or duplication.
- **simple output**: one json object per event on stdout, ready for tools like [vector](https://vector.dev) to forward to [Elasticsearch](https://www.elastic.co/elasticsearch) or [kafka](https://kafka.apache.org/documentation/).

## features

- multi-service support for exporting logs from AWS and custom applications.
- one json object per event on stdout; a shipper such as vector forwards it to files, kafka or elasticsearch.
- offset management using consul KV to ensure logs are processed consistently and to ensure delivery atleast once.
- concurrent processing of multiple log streams for efficiency.
- configuration through a YAML file.

## architecture


1. **configuration loading**: settings are loaded from a YAML file, including AWS credentials and service details.
2. **consul client setup**: connection to consul is established to manage and persist log stream offsets.
3. **log stream discovery**: relevant log streams are discovered based on the provided configuration.
4. **log taling**: Goroutines are initiated to fetch and process log events in real-time.
5. **offset management**: processed logs are saved back to consul KV store to maintain state across restarts.
6. **log output**: each event is written to stdout as one json line; diagnostics go to stderr.

## prerequisites

before deploying and running the **cwsync**, ensure the following:

- **AWS Credentials**: AWS creds with appropriate permissions to access cloudwatch Logs.
- **consul**: A running consul instance accessible for offset management.
- **a log shipper**: a tool that reads cwsync's stdout and forwards it, such as vector.

## configuration

The application is configured using a YAML file (`config.yaml`). Below is a sample configuration:

```yaml
consul:
  address: "http://localhost:8500"
  token: "your-consul-token"

aws_region: "us-east-1"
aws_profile: "default"
aws_role_arn: "arn:aws:iam::123456789012:role/role"
aws_access_key: "YOUR_ACCESS_KEY"
aws_secret_key: "YOUR_SECRET_KEY"

services:
  - name: "my-service"
    consul_kv_path: "log_offsets/my-service"
    log_configs:
      - log_group_name: "/aws/lambda/my-function"
        log_stream_prefix: "function-name"
```
### configuration parameters
- consul configuration:
  - consul.address: The http address of your consul server.
  - consul.token: The access token for consul.
- AWS Configuration:
  - aws_region: AWS region for CloudWatch logs.
  - aws_profile: (optional) AWS CLI profile for credentials.
  - aws_role_arn: (optional) ARN of the AWS IAM role to assume.
  - aws_access_key & aws_secret_key: (optional) Static AWS credentials.
- Services Configuration:
  - services: list of services to monitor and export logs for.
  - name: identifier for the service.
  - consul_kv_path: consul KV path for saving log offsets.
  - log_configs: list of log groups and streams to monitor.

## output

each cloudwatch event is written to stdout as one json object per line. everything else (startup notes, errors) goes to stderr, so stdout is safe to read line by line.

```json
{"stream":"my-stream","timestamp":1730000000000,"ingestion_time":1730000001000,"message":"raw event text"}
```

| field | meaning |
|---|---|
| stream | log stream name |
| timestamp | event time in epoch milliseconds, set by the producer |
| ingestion_time | when cloudwatch received the event, in epoch milliseconds |
| message | the event text, unchanged. newlines stay escaped inside the string, so a multi-line record is still one line |

delivery is at least once: after a restart an event can be written again.

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
