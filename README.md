mackerel-plugin-aws-kinesis-streams-shard
=========================================

AWS Kinesis Streams `Shard-Level` custom metrics plugin for mackerel.io agent.

## Synopsis

```shell
mackerel-plugin-aws-kinesis-streams-shard -identifier=<stream-name> -region=<aws-region> [-access-key-id=<id>] [-secret-access-key=<key>] [-tempfile=<tempfile>]
```
* collect data from specified AWS Kinesis Streams
* you can set keys by environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`

## Example of mackerel-agent.conf

```
[plugin.metrics.aws-kinesis-streams-shard]
command = "/path/to/mackerel-plugin-aws-kinesis-streams-shard"
```
