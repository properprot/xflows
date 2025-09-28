package outputs

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/properprot/xflows/pkg/sflow"
	"github.com/sirupsen/logrus"
)

type ClickHouseConfig struct {
	DSN           string            `yaml:"dsn"`
	Database      string            `yaml:"database"`
	Username      string            `yaml:"username"`
	Password      string            `yaml:"password"`
	Table         string            `yaml:"table"`
	BatchSize     int               `yaml:"batch_size"`
	FlushInterval time.Duration     `yaml:"flush_interval"`
	Columns       map[string]string `yaml:"columns"`    // sample fields to column names
	CustomSQL     string            `yaml:"custom_sql"` // custom insert SQL template
	TLS           bool              `yaml:"tls"`
}

type ClickHouseOutput struct {
	conn       clickhouse.Conn
	config     ClickHouseConfig
	batch      []*sflow.SFlowSample
	mutex      sync.Mutex
	flushTimer *time.Timer
	logger     *logrus.Logger
	insertSQL  string
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewClickHouseOutput(config map[string]interface{}, level logrus.Level) (*ClickHouseOutput, error) {
	cfg := ClickHouseConfig{
		BatchSize:     1000,
		FlushInterval: 1 * time.Second,
		Table:         "sflow_samples",
	}

	if dsn, ok := config["dsn"].(string); ok {
		cfg.DSN = dsn
	} else {
		return nil, fmt.Errorf("clickhouse dsn not specified")
	}

	if db, ok := config["database"].(string); ok {
		cfg.Database = db
	}

	if user, ok := config["username"].(string); ok {
		cfg.Username = user
	}

	if pass, ok := config["password"].(string); ok {
		cfg.Password = pass
	}

	if table, ok := config["table"].(string); ok {
		cfg.Table = table
	}

	if size, ok := config["batch_size"].(int); ok {
		cfg.BatchSize = size
	}

	if interval, ok := config["flush_interval"].(string); ok {
		if d, err := time.ParseDuration(interval); err == nil {
			cfg.FlushInterval = d
		}
	}

	if customSQL, ok := config["custom_sql"].(string); ok {
		cfg.CustomSQL = customSQL
	}

	if columns, ok := config["columns"].(map[string]interface{}); ok {
		cfg.Columns = make(map[string]string)
		for k, v := range columns {
			if s, ok := v.(string); ok {
				cfg.Columns[k] = s
			}
		}
	}

	if tls, ok := config["tls"].(bool); ok {
		cfg.TLS = tls
	}

	// Connect to ClickHouse
	tls_setting := &tls.Config{InsecureSkipVerify: true}
	if !cfg.TLS {
		tls_setting = nil
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.DSN},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 30 * time.Second,
		TLS:         tls_setting,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	log.SetLevel(level)
	output := &ClickHouseOutput{
		conn:   conn,
		config: cfg,
		batch:  make([]*sflow.SFlowSample, 0, cfg.BatchSize),
		logger: log,
		ctx:    ctx,
		cancel: cancel,
	}

	output.insertSQL = output.buildInsertSQL()

	if err := output.ensureTable(); err != nil {
		return nil, fmt.Errorf("failed to ensure table exists: %w", err)
	}

	output.resetFlushTimer()

	return output, nil
}

func (c *ClickHouseOutput) buildInsertSQL() string {
	if c.config.CustomSQL != "" {
		return c.config.CustomSQL
	}

	columns := []string{
		c.getColumnName("timestamp", "timestamp"),
		c.getColumnName("agent_ip", "agent_ip"),
		c.getColumnName("sample_rate", "sample_rate"),
		c.getColumnName("payload", "payload"),
		c.getColumnName("packet_count", "packet_count"),
		c.getColumnName("byte_count", "byte_count"),
		c.getColumnName("packet_size", "packet_size"),
		c.getColumnName("tos", "tos"),
		c.getColumnName("src_ip", "src_ip"),
		c.getColumnName("dst_ip", "dst_ip"),
		c.getColumnName("protocol", "protocol"),
		c.getColumnName("src_port", "src_port"),
		c.getColumnName("dst_port", "dst_port"),
	}

	placeholders := strings.Repeat("?,", len(columns))
	placeholders = placeholders[:len(placeholders)-1] // remove trailing comma

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		c.config.Table,
		strings.Join(columns, ","),
		placeholders,
	)
}

func (c *ClickHouseOutput) getColumnName(field, defaultName string) string {
	if customName, ok := c.config.Columns[field]; ok {
		return customName
	}
	return defaultName
}

func (c *ClickHouseOutput) ensureTable() error {
	createTableSQL := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            timestamp DateTime64(9),
            agent_ip IPv4,
            sample_rate UInt64,
            payload String,
            packet_count UInt64,
            byte_count UInt64,
            packet_size UInt32,
            tos UInt8,
            src_ip IPv4,
            dst_ip IPv4,
            protocol UInt8,
            src_port UInt16,
            dst_port UInt16
        ) ENGINE = MergeTree()
        ORDER BY (timestamp, src_ip, dst_ip)
        SETTINGS index_granularity = 8192
    `, c.config.Table)

	return c.conn.Exec(context.Background(), createTableSQL)
}

func (c *ClickHouseOutput) Send(sample *sflow.SFlowSample) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.batch = append(c.batch, sample)

	if len(c.batch) >= c.config.BatchSize {
		return c.flush()
	}

	return nil
}

func (c *ClickHouseOutput) flush() error {
	if len(c.batch) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(c.ctx, c.insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, sample := range c.batch {
		if err := c.appendSampleToBatch(batch, sample); err != nil {
			return fmt.Errorf("failed to append sample to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	c.logger.WithField("batch_size", len(c.batch)).Debug("Successfully sent batch to ClickHouse")
	c.batch = c.batch[:0]
	c.resetFlushTimer()

	return nil
}

func (c *ClickHouseOutput) appendSampleToBatch(batch driver.Batch, sample *sflow.SFlowSample) error {
	return batch.Append(
		sample.Timestamp,
		sample.AgentIP.String(),
		sample.SampleRate,
		sample.Payload,
		sample.PacketCount,
		sample.ByteCount,
		sample.PacketSize,
		sample.TOS,
		sample.SrcIP.String(),
		sample.DstIP.String(),
		sample.Protocol,
		sample.SrcPort,
		sample.DstPort,
	)
}

func (c *ClickHouseOutput) resetFlushTimer() {
	if c.flushTimer != nil {
		c.flushTimer.Stop()
	}

	c.flushTimer = time.AfterFunc(c.config.FlushInterval, func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()

		if len(c.batch) > 0 {
			if err := c.flush(); err != nil {
				c.logger.WithError(err).Error("Failed to flush batch on timer")
			}
		}
	})
}

func (c *ClickHouseOutput) Close() error {
	c.cancel()

	c.mutex.Lock()
	if len(c.batch) > 0 {
		if err := c.flush(); err != nil {
			c.logger.WithError(err).Error("Failed to flush final batch")
		}
	}
	c.mutex.Unlock()

	if c.flushTimer != nil {
		c.flushTimer.Stop()
	}

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *ClickHouseOutput) Flush() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.flush()
}
