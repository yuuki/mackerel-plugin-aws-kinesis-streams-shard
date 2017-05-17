package mpawskinesisstreamsshard

import (
	"errors"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/kinesis"
	mp "github.com/mackerelio/go-mackerel-plugin-helper"
)

const (
	namespace          = "AWS/Kinesis"
	metricsTypeAverage = "Average"
	metricsTypeMaximum = "Maximum"
	metricsTypeMinimum = "Minimum"
)

type metrics struct {
	CloudWatchName string
	MackerelName   string
	Type           string
	GraphdefPrefix string
}

// KinesisStreamsShardPlugin mackerel plugin for aws kinesis
type KinesisStreamsShardPlugin struct {
	Name   string
	Prefix string

	AccessKeyID     string
	SecretAccessKey string
	Region          string
	CloudWatch      *cloudwatch.CloudWatch
	Kinesis         *kinesis.Kinesis
}

// MetricKeyPrefix interface for PluginWithPrefix
func (p KinesisStreamsShardPlugin) MetricKeyPrefix() string {
	if p.Prefix == "" {
		p.Prefix = "kinesis-streams-shard"
	}
	return p.Prefix
}

// prepare creates CloudWatch instance
func (p *KinesisStreamsShardPlugin) prepare() error {
	sess, err := session.NewSession()
	if err != nil {
		return err
	}

	config := aws.NewConfig()
	if p.AccessKeyID != "" && p.SecretAccessKey != "" {
		config = config.WithCredentials(credentials.NewStaticCredentials(p.AccessKeyID, p.SecretAccessKey, ""))
	}
	if p.Region != "" {
		config = config.WithRegion(p.Region)
	}

	p.CloudWatch = cloudwatch.New(sess, config)
	p.Kinesis = kinesis.New(sess, config)

	return nil
}

// getLastPoint fetches a CloudWatch metric and parse
func (p KinesisStreamsShardPlugin) getLastPoint(metric metrics, shardID string) (float64, error) {
	now := time.Now()

	dimensions := []*cloudwatch.Dimension{
		{
			Name:  aws.String("StreamName"),
			Value: aws.String(p.Name),
		},
		{
			Name:  aws.String("ShardId"),
			Value: aws.String(shardID),
		},
	}

	response, err := p.CloudWatch.GetMetricStatistics(&cloudwatch.GetMetricStatisticsInput{
		Dimensions: dimensions,
		StartTime:  aws.Time(now.Add(time.Duration(180) * time.Second * -1)), // 3 min
		EndTime:    aws.Time(now),
		MetricName: aws.String(metric.CloudWatchName),
		Period:     aws.Int64(60),
		Statistics: []*string{aws.String(metric.Type)},
		Namespace:  aws.String(namespace),
	})
	if err != nil {
		return 0, err
	}

	datapoints := response.Datapoints
	if len(datapoints) == 0 {
		return 0, errors.New("fetched no datapoints")
	}

	latest := new(time.Time)
	var latestVal float64
	for _, dp := range datapoints {
		if dp.Timestamp.Before(*latest) {
			continue
		}

		latest = dp.Timestamp
		switch metric.Type {
		case metricsTypeAverage:
			latestVal = *dp.Average
		case metricsTypeMaximum:
			latestVal = *dp.Maximum
		case metricsTypeMinimum:
			latestVal = *dp.Minimum
		}
	}

	return latestVal, nil
}

// FetchMetrics fetch the metrics
func (p KinesisStreamsShardPlugin) FetchMetrics() (map[string]interface{}, error) {
	stat := make(map[string]interface{})

	shardIDs, err := p.GetShardIDs()
	if err != nil {
		return stat, err
	}
	for _, shardID := range shardIDs {
		for _, met := range [...]metrics{
			{CloudWatchName: "OutgoingBytes", MackerelName: "GetRecordsBytes", Type: metricsTypeAverage, GraphdefPrefix: "bytes"},
			// Max of IteratorAgeMilliseconds is useful especially when few of iterators are in trouble
			{CloudWatchName: "IteratorAgeMilliseconds", MackerelName: "GetRecordsDelayMaxMilliseconds", Type: metricsTypeMaximum, GraphdefPrefix: "iteratorage"},
			{CloudWatchName: "IteratorAgeMilliseconds", MackerelName: "GetRecordsDelayMinMilliseconds", Type: metricsTypeMinimum, GraphdefPrefix: "iteratorage"},
			{CloudWatchName: "IteratorAgeMilliseconds", MackerelName: "GetRecordsDelayAverageMilliseconds", Type: metricsTypeAverage, GraphdefPrefix: "iteratorage"},
			{CloudWatchName: "OutgoingRecords", MackerelName: "GetRecordsRecords", Type: metricsTypeAverage, GraphdefPrefix: "records"},
			{CloudWatchName: "IncomingBytes", MackerelName: "IncomingBytes", Type: metricsTypeAverage, GraphdefPrefix: "bytes"},
			{CloudWatchName: "IncomingRecords", MackerelName: "IncomingRecords", Type: metricsTypeAverage, GraphdefPrefix: "records"},
			{CloudWatchName: "ReadProvidionedThroughputExceeded", MackerelName: "ReadThroughputExceeded", Type: metricsTypeAverage, GraphdefPrefix: "pending"},
			{CloudWatchName: "WriteProvidionedThroughputExceeded", MackerelName: "WriteThroughputExceeded", Type: metricsTypeAverage, GraphdefPrefix: "pending"},
		} {
			v, err := p.getLastPoint(met, shardID)
			if err == nil {
				stat[met.GraphdefPrefix+"."+shardID+"."+met.MackerelName] = v
			} else {
				log.Printf("%s %s: %s", shardID, met, err)
			}
		}
	}
	return stat, nil
}

// GraphDefinition of KinesisStreamsShardPlugin
func (p KinesisStreamsShardPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := strings.Title(p.Prefix)
	labelPrefix = strings.Replace(labelPrefix, "-", " ", -1)

	var graphdef = map[string]mp.Graphs{
		"bytes.#": {
			Label: (labelPrefix + " Bytes"),
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "GetRecordsBytes", Label: "GetRecords"},
				{Name: "IncomingBytes", Label: "Total Incoming"},
			},
		},
		"iteratorage.#": {
			Label: (labelPrefix + " Read Delay"),
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "GetRecordsDelayAverageMilliseconds", Label: "Average"},
				{Name: "GetRecordsDelayMaxMilliseconds", Label: "Max"},
				{Name: "GetRecordsDelayMinMilliseconds", Label: "Min"},
			},
		},
		"records.#": {
			Label: (labelPrefix + " Records"),
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "GetRecordsRecords", Label: "GetRecords"},
				{Name: "IncomingRecords", Label: "Total Incoming"},
			},
		},
		"pending.#": {
			Label: (labelPrefix + " Pending Operations"),
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "ReadThroughputExceeded", Label: "Read"},
				{Name: "WriteThroughputExceeded", Label: "Write"},
			},
		},
	}
	return graphdef
}

// GetShards gets shardid
func (p *KinesisStreamsShardPlugin) GetShardIDs() ([]string, error) {
	resp, err := p.Kinesis.DescribeStream(&kinesis.DescribeStreamInput{
		StreamName: aws.String(p.Name),
	})
	if err != nil {
		return nil, err
	}
	shards := resp.StreamDescription.Shards
	shardIDs := make([]string, 0, len(shards))
	for _, shard := range shards {
		shardIDs = append(shardIDs, *shard.ShardId)
	}
	return shardIDs, nil
}

// Do the plugin
func Do() {
	optAccessKeyID := flag.String("access-key-id", "", "AWS Access Key ID")
	optSecretAccessKey := flag.String("secret-access-key", "", "AWS Secret Access Key")
	optRegion := flag.String("region", "", "AWS Region")
	optIdentifier := flag.String("identifier", "", "Stream Name")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	optPrefix := flag.String("metric-key-prefix", "kinesis-streams-shard", "Metric key prefix")
	flag.Parse()

	var plugin KinesisStreamsShardPlugin

	plugin.AccessKeyID = *optAccessKeyID
	plugin.SecretAccessKey = *optSecretAccessKey
	plugin.Region = *optRegion
	plugin.Name = *optIdentifier
	plugin.Prefix = *optPrefix

	err := plugin.prepare()
	if err != nil {
		log.Fatalln(err)
	}

	helper := mp.NewMackerelPlugin(plugin)
	helper.Tempfile = *optTempfile

	helper.Run()
}
