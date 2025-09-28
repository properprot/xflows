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

	// Datagram header
	binary.Write(buf, binary.BigEndian, uint32(5)) // version
	binary.Write(buf, binary.BigEndian, uint32(1)) // agent addr type = IPv4
	binary.Write(buf, binary.BigEndian, s.ipToUint32(s.agentIP))
	binary.Write(buf, binary.BigEndian, s.subAgentID)
	binary.Write(buf, binary.BigEndian, atomic.AddUint32(&s.sequenceNumber, 1))
	binary.Write(buf, binary.BigEndian, uint32(time.Since(s.bootTime).Milliseconds()))
	binary.Write(buf, binary.BigEndian, uint32(1))

	// Add one flow sample
	if err := s.writeSampleRecord(buf, sample); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SFlowCollectorOutput) writeSampleRecord(buf *bytes.Buffer, sample *sflow.SFlowSample) error {
	sampleBuf := &bytes.Buffer{}

	binary.Write(sampleBuf, binary.BigEndian, uint32(s.sequenceNumber))
	binary.Write(sampleBuf, binary.BigEndian, uint32(0<<24|1))
	binary.Write(sampleBuf, binary.BigEndian, uint32(sample.SampleRate))
	binary.Write(sampleBuf, binary.BigEndian, uint32(sample.PacketCount)) // sample pool
	binary.Write(sampleBuf, binary.BigEndian, uint32(0))
	binary.Write(sampleBuf, binary.BigEndian, uint32(1)) // we dont know these ifIndexs
	binary.Write(sampleBuf, binary.BigEndian, uint32(1))
	binary.Write(sampleBuf, binary.BigEndian, uint32(1)) // number of records

	flowRec := &bytes.Buffer{}
	binary.Write(flowRec, binary.BigEndian, uint32(1)) // type = HDR (format=1, enterprise=0)

	flowData := &bytes.Buffer{}
	binary.Write(flowData, binary.BigEndian, uint32(1)) // headerProtocol = 1 (Ethernet)
	binary.Write(flowData, binary.BigEndian, uint32(sample.PacketSize))
	binary.Write(flowData, binary.BigEndian, uint32(0)) // stripped = 0
	binary.Write(flowData, binary.BigEndian, uint32(len(sample.Raw)))
	flowData.Write(sample.Raw)

	for flowData.Len()%4 != 0 {
		flowData.WriteByte(0)
	}

	binary.Write(flowRec, binary.BigEndian, uint32(flowData.Len()))
	flowRec.Write(flowData.Bytes())

	sampleBuf.Write(flowRec.Bytes())

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
