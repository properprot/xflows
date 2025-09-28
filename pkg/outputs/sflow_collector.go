package outputs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/properprot/xflows/pkg/sflow"
	"github.com/sirupsen/logrus"
)

type SFlowCollectorOutput struct {
	conn           net.Conn
	collectorAddr  string
	sequenceNumber uint32
	agentIP        net.IP
	subAgentID     uint32
	bootTime       time.Time
	logger         *logrus.Logger
}

func NewSFlowCollectorOutput(config map[string]interface{}, level logrus.Level) (*SFlowCollectorOutput, error) {
	addr, ok := config["collector_address"].(string)
	if !ok {
		return nil, fmt.Errorf("sflow collector_address not specified")
	}

	agentIPStr, ok := config["agent_ip"].(string)
	if !ok {
		return nil, fmt.Errorf("sflow agent_ip not specified")
	}

	agentIP := net.ParseIP(agentIPStr)
	if agentIP == nil {
		return nil, fmt.Errorf("invalid agent IP: %s", agentIPStr)
	}

	subAgentID := uint32(1)
	if id, ok := config["sub_agent_id"].(int); ok {
		subAgentID = uint32(id)
	}

	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sFlow collector: %w", err)
	}

	log.SetLevel(level)
	return &SFlowCollectorOutput{
		conn:           conn,
		collectorAddr:  addr,
		sequenceNumber: 0,
		agentIP:        agentIP,
		subAgentID:     subAgentID,
		bootTime:       time.Now(),
		logger:         log,
	}, nil
}

func (s *SFlowCollectorOutput) Send(sample *sflow.SFlowSample) error {
	datagram, err := s.buildSFlowV5(sample)
	if err != nil {
		return fmt.Errorf("failed to build sFlow datagram: %w", err)
	}

	_, err = s.conn.Write(datagram)
	if err != nil {
		return fmt.Errorf("failed to send sFlow datagram: %w", err)
	}

	return nil
}

func (s *SFlowCollectorOutput) buildSFlowV5(sample *sflow.SFlowSample) ([]byte, error) {
	buf := &bytes.Buffer{}

	binary.Write(buf, binary.BigEndian, uint32(5)) // Vers
	binary.Write(buf, binary.BigEndian, s.ipToUint32(s.agentIP))
	binary.Write(buf, binary.BigEndian, s.subAgentID)
	binary.Write(buf, binary.BigEndian, atomic.AddUint32(&s.sequenceNumber, 1))
	binary.Write(buf, binary.BigEndian, uint32(time.Since(s.bootTime).Milliseconds()))
	binary.Write(buf, binary.BigEndian, uint32(1)) // SampleCount

	if err := s.writeSampleRecord(buf, sample); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SFlowCollectorOutput) writeSampleRecord(buf *bytes.Buffer, sample *sflow.SFlowSample) error {
	sampleBuf := &bytes.Buffer{}

	// Flow Sample HDR
	binary.Write(sampleBuf, binary.BigEndian, uint32(sample.SampleRate))
	binary.Write(sampleBuf, binary.BigEndian, uint32(sample.PacketCount)) // Sample pool (we don't read this)
	binary.Write(sampleBuf, binary.BigEndian, uint32(0))                  // We don't read drops
	binary.Write(sampleBuf, binary.BigEndian, uint32(0))                  // or input interface
	binary.Write(sampleBuf, binary.BigEndian, uint32(0))                  // or output interface
	binary.Write(sampleBuf, binary.BigEndian, uint32(1))                  // SampleCount

	flowRecordBuf := &bytes.Buffer{}
	binary.Write(flowRecordBuf, binary.BigEndian, uint32(1)) // Format, raw packet

	flowDataBuf := &bytes.Buffer{}
	binary.Write(flowDataBuf, binary.BigEndian, uint32(1)) // ETH Proto
	binary.Write(flowDataBuf, binary.BigEndian, uint32(sample.PacketSize))
	binary.Write(flowDataBuf, binary.BigEndian, uint32(sample.PacketSize)-uint32(len(sample.Raw))) // Payload removed (we have full header)
	binary.Write(flowDataBuf, binary.BigEndian, uint32(len(sample.Raw)))
	flowDataBuf.Write(sample.Raw)

	binary.Write(flowRecordBuf, binary.BigEndian, uint32(flowDataBuf.Len()))
	flowRecordBuf.Write(flowDataBuf.Bytes())

	sampleBuf.Write(flowRecordBuf.Bytes())

	binary.Write(buf, binary.BigEndian, uint32(1))
	binary.Write(buf, binary.BigEndian, uint32(sampleBuf.Len()))
	buf.Write(sampleBuf.Bytes())

	return nil
}

func (s *SFlowCollectorOutput) ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return binary.BigEndian.Uint32(ip)
}

func (s *SFlowCollectorOutput) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
