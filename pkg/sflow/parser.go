package sflow

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type Parser struct {
	agentIP    net.IP
	sampleRate uint64
}

func NewParser(agentIP string, sampleRate uint64) (*Parser, error) {
	ip := net.ParseIP(agentIP)
	if ip == nil {
		return nil, fmt.Errorf("invalid agent IP: %s", agentIP)
	}

	return &Parser{
		agentIP:    ip,
		sampleRate: sampleRate,
	}, nil
}

func (p *Parser) ParseEBPFData(data []byte) (*SFlowSample, error) {
	if len(data) != 128 { // We expect 128, exactly
		return nil, fmt.Errorf("insufficient data length: %d", len(data))
	}

	// Grab Timestamp, AgentIP, SampleRate, PacketSize, SrcIP, DstIP, SrcPort, DstPort, Protocol, TOS, PacketCount (s.rate), ByteCount (ip len*s.rate)
	// for the SFlowSample we want to return

	_ipOffset := 14
	_ethType := binary.BigEndian.Uint16(data[12:14])

	// Support 802.1Q VLAN Tag
	if _ethType == 0x8100 {
		_ethType = binary.BigEndian.Uint16(data[16:18])
		_ipOffset += 4
	}

	// Should only be IPv4 at this point
	if _ethType != 0x0800 {
		return nil, fmt.Errorf("unsupported ethertype: %x", _ethType)
	}

	_ipPacket := data[_ipOffset:]
	if _ipPacket[0]>>4 != 4 {
		return nil, fmt.Errorf("unsupported IP version: %d", _ipPacket[0]>>4)
	}

	_totalLen := int(binary.BigEndian.Uint16(_ipPacket[2:4])) + 14 // Ethernet HDR

	var srcPort, dstPort uint16
	if _ipPacket[9] == 6 || _ipPacket[9] == 17 {
		_transportPacket := _ipPacket[int(_ipPacket[0]&0x0F)*4:]

		srcPort = binary.BigEndian.Uint16(_transportPacket[0:2])
		dstPort = binary.BigEndian.Uint16(_transportPacket[2:4])
	} else {
		srcPort = 0
		dstPort = 0
	}

	_cutoff := _totalLen
	if _cutoff > 127 { // Total sent via XDP, if a packet is smaller, we can adjust to that
		_cutoff = 127
	}

	return &SFlowSample{
		Timestamp:   time.Now(),
		AgentIP:     p.agentIP,
		SampleRate:  p.sampleRate,
		Payload:     fmt.Sprintf("%x", data[:_cutoff]),
		PacketCount: p.sampleRate,
		ByteCount:   uint64(_totalLen) * p.sampleRate,
		PacketSize:  uint32(_totalLen),
		Raw:         data[:_cutoff],

		TOS:      uint8(_ipPacket[8]),
		SrcIP:    net.IP(_ipPacket[12:16]),
		DstIP:    net.IP(_ipPacket[16:20]),
		Protocol: uint8(_ipPacket[9]),

		SrcPort: srcPort,
		DstPort: dstPort,
	}, nil
}
