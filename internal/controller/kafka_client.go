package controller

import (
	"context"
	"fmt"
	"os"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type LagFetcher interface {
	GetConsumerLag(
		ctx context.Context,
		brokers []string,
		topic string,
		consumerGroup string,
	) (int64, error)
}

type KafkaClient struct{}

func NewKafkaClient() *KafkaClient {
	return &KafkaClient{}
}

func (k *KafkaClient) GetConsumerLag(
	ctx context.Context,
	brokers []string,
	topic string,
	consumerGroup string,
) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.WithLogger(kgo.BasicLogger(os.Stderr, kgo.LogLevelWarn, nil)),
	)
	if err != nil {
		return 0, fmt.Errorf("creating kafka client: %w", err)
	}
	defer client.Close()

	kadmClient := kadm.NewClient(client)

	endOffsets, err := kadmClient.ListEndOffsets(ctx, topic)
	if err != nil {
		return 0, fmt.Errorf("listing end offsets: %w", err)
	}

	topicEndOffsets, ok := endOffsets[topic]
	if !ok || len(topicEndOffsets) == 0 {
		return 0, nil
	}

	if len(topicEndOffsets) == 1 {
		if _, ok := topicEndOffsets[-1]; ok {
			return 0, nil
		}
	}

	committedOffsets, err := kadmClient.FetchOffsets(ctx, consumerGroup)
	if err != nil {
		return 0, fmt.Errorf("fetching committed offsets: %w", err)
	}

	topicCommittedOffsets := committedOffsets[topic]
	var totalLag int64

	for partition, endOffset := range topicEndOffsets {
		if partition < 0 {
			continue
		}

		if endOffset.Err != nil {
			return 0, fmt.Errorf("listing end offsets for topic %q partition %d: %w", topic, partition, endOffset.Err)
		}

		committedOffset := int64(0)
		if topicCommittedOffsets != nil {
			if committed, ok := topicCommittedOffsets[partition]; ok {
				if committed.Err != nil {
					return 0, fmt.Errorf("fetching committed offsets for topic %q partition %d: %w", topic, partition, committed.Err)
				}
				if committed.At >= 0 {
					committedOffset = committed.At
				}
			}
		}

		lag := endOffset.Offset - committedOffset
		if lag < 0 {
			lag = 0
		}

		totalLag += lag
	}

	return totalLag, nil
}
