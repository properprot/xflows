package sflow

import (
	"net"
	"time"
)

type SFlowSample struct {
	// Generic
	Timestamp   time.Time
	AgentIP     net.IP
	SampleRate  uint64
	Payload     string
	PacketCount uint64
	ByteCount   uint64
	PacketSize  uint32

	// L3
	TOS      uint8
	SrcIP    net.IP
	DstIP    net.IP
	Protocol uint8

	// L4
	SrcPort uint16
	DstPort uint16
}

type RawEBPFOut struct {
	Bytes [128]byte
}
