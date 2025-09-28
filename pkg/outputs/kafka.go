package outputs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/properprot/xflows/pkg/sflow"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type KafkaOutput struct {
	writer *kafka.Writer
	logger *logrus.Logger
}

func NewKafkaOutput(config map[string]interface{}, level logrus.Level) (*KafkaOutput, error) {
	brokers, ok := config["brokers"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("kafka brokers not specified")
	}

	topic, ok := config["topic"].(string)
	if !ok {
		return nil, fmt.Errorf("kafka topic not specified")
	}

	brokerStrs := make([]string, len(brokers))
	for i, broker := range brokers {
		brokerStrs[i] = broker.(string)
	}

	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokerStrs...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	log.SetLevel(level)
	return &KafkaOutput{writer: writer, logger: log}, nil
}

func (k *KafkaOutput) Send(sample *sflow.SFlowSample) error {
	data, err := json.Marshal(sample)
	if err != nil {
		return nil
	}

	return k.writer.WriteMessages(context.Background(), kafka.Message{Value: data})
}

func (k *KafkaOutput) Close() error {
	return k.writer.Close()
}
