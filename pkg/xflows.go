package xflows

import (
	"context"
	"fmt"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/properprot/xflows/pkg/config"
	"github.com/properprot/xflows/pkg/outputs"
	"github.com/properprot/xflows/pkg/sflow"
	"github.com/properprot/xflows/pkg/xdp"
	"github.com/sirupsen/logrus"
)

type Exporter struct {
	config  *config.Config
	outputs []outputs.Output
	logger  *logrus.Logger
	parser  *sflow.Parser
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewExporter(configPath string) (*Exporter, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger := logrus.New()
	if level, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logger.SetLevel(level)
	}

	return NewExporterWithConfig(cfg, logger)
}

func NewExporterWithConfig(cfg *config.Config, logger *logrus.Logger) (*Exporter, error) {
	if logger == nil {
		logger = logrus.New()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// sFlow parser
	parser, err := sflow.NewParser(cfg.SFlow.AgentIP, cfg.SFlow.SampleRate)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create sFlow parser: %w", err)
	}

	// Initialise outputs
	var outputInstances []outputs.Output
	for _, outputCfg := range cfg.Outputs {
		var output outputs.Output
		switch outputCfg.Type {
		case "console":
			output, err = outputs.NewConsoleOutput(outputCfg.Config, cfg.LogrusLevel)
		case "kafka":
			output, err = outputs.NewKafkaOutput(outputCfg.Config, cfg.LogrusLevel)
		case "clickhouse":
			output, err = outputs.NewClickHouseOutput(outputCfg.Config, cfg.LogrusLevel)
		case "sflow_collector":
			output, err = outputs.NewSFlowCollectorOutput(outputCfg.Config, cfg.LogrusLevel)
		default:
			err = fmt.Errorf("unknown output type: %s", outputCfg.Type)
		}

		if err != nil {
			cancel()
			for _, o := range outputInstances {
				o.Close()
			}
			return nil, fmt.Errorf("failed to create %s output: %w", outputCfg.Type, err)
		}
		outputInstances = append(outputInstances, output)
	}

	return &Exporter{
		config:  cfg,
		outputs: outputInstances,
		logger:  logger,
		parser:  parser,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func (e *Exporter) ListenFlows(ringBufferMap *ebpf.Map) error {
	dataChan := make(chan []byte, e.config.Processing.ChannelBuffer)

	mapReader := xdp.NewMapReader(ringBufferMap)

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer close(dataChan)

		e.logger.Info("Starting XFlows ring buffer monitoring")
		mapReader.Start(e.ctx, dataChan)
		e.logger.Info("XFlows ring buffer stream stopped")
	}()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()

		e.logger.Info("Starting XFlows processor")

		for {
			select {
			case <-e.ctx.Done():
				e.logger.Info("XFlows processor shutting down")
				return
			case rawData, ok := <-dataChan:
				if !ok {
					e.logger.Info("Data channel closed, processor stopping")
					return
				}

				sample, err := e.parser.ParseEBPFData(rawData)
				if err != nil {
					e.logger.WithError(err).Error("Failed to parse eBPF data")
					continue
				}

				if e.config.Processing.ConcurrentSends {
					for i, output := range e.outputs {
						go func(idx int, out outputs.Output, s *sflow.SFlowSample) {
							if err := out.Send(s); err != nil {
								e.logger.WithError(err).WithField("output_index", idx).Error("Failed to send to output")
							}
						}(i, output, sample)
					}
				} else {
					for i, output := range e.outputs {
						if err := output.Send(sample); err != nil {
							e.logger.WithError(err).WithField("output_index", i).Error("Failed to send to output")
						}
					}
				}
			}
		}
	}()

	e.logger.Info("XFlows started and processing flows")
	return nil
}

func (e *Exporter) Stop() {
	e.logger.Info("Stopping XFlows exporter")
	e.cancel()
	e.wg.Wait()
	e.logger.Info("XFlows exporter stopped")
}

func (e *Exporter) Close() error {
	e.Stop()

	// Close all outputs
	for i, output := range e.outputs {
		if err := output.Close(); err != nil {
			e.logger.WithError(err).WithField("output_index", i).Error("Failed to close output")
		}
	}

	return nil
}

// Grabs current cfg
func (e *Exporter) Config() *config.Config {
	return e.config
}

// If a user wants to add custom logging in
func (e *Exporter) Logger() *logrus.Logger {
	return e.logger
}
