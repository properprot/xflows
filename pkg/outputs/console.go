package outputs

import (
	"encoding/json"
	"fmt"

	"github.com/properprot/xflows/pkg/sflow"
	"github.com/sirupsen/logrus"
)

type ConsoleOutput struct {
	format string
	logger *logrus.Logger
}

func NewConsoleOutput(config map[string]interface{}, level logrus.Level) (*ConsoleOutput, error) {
	format := "json"
	if f, ok := config["format"].(string); ok {
		format = f
	}

	if format != "json" && format != "text" {
		format = "json"
	}

	log.SetLevel(level)
	return &ConsoleOutput{format: format, logger: log}, nil
}

func (c *ConsoleOutput) Send(sample *sflow.SFlowSample) error {
	switch c.format {
	case "json":
		data, err := json.Marshal(sample)
		if err != nil {
			return err
		}
		c.logger.Info(string(data))
	case "text":
		c.logger.Info(fmt.Sprintf("sFlow: %s:%d -> %s:%d (proto=%d, size=%d bytes, s.rate=%d)",
			sample.SrcIP, sample.SrcPort,
			sample.DstIP, sample.DstPort,
			sample.Protocol, sample.PacketSize, sample.SampleRate,
		))
	}

	return nil
}

func (c *ConsoleOutput) Close() error {
	return nil
}
