package xdp

import (
	"context"
	"fmt"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type MapReader struct {
	ebpfMap *ebpf.Map
	logger  *logrus.Logger
}

func NewMapReader(SFlowMap *ebpf.Map) *MapReader {
	return &MapReader{
		ebpfMap: SFlowMap,
		logger:  log,
	}
}

func (mr *MapReader) Start(ctx context.Context, dataChan chan<- []byte) {
	mr.logger.Debug("Beginning map reader")

	ringbuf_reader, err := ringbuf.NewReader(mr.ebpfMap)
	if err != nil {
		mr.logger.Error(fmt.Sprintf("Failed to create ringbuf reader: %v", err))
		return
	}
	defer ringbuf_reader.Close()
	ringbuf_reader.SetDeadline(time.Now().Add(10 * time.Second))

	for {
		select {
		case <-ctx.Done():
			mr.logger.Info("Map reader shutting down")
			return
		default:
			for {
				msg, err := ringbuf_reader.Read()
				if err != nil {
					if err.Error() == "epoll wait: i/o timeout" {
						if _, err := mr.ebpfMap.Info(); err != nil {
							mr.logger.Error("Ringbuf map has expired")
							return
						}

						ringbuf_reader.SetDeadline(time.Now().Add(5 * time.Second))
						break
					}

					mr.logger.Error(fmt.Sprintf("Failed to read from ringbuf: %v", err))
					return
				}

				if len(msg.RawSample) != 128 {
					mr.logger.Error(fmt.Sprintf("Invalid data length (%d) in ringbuf, you have likely setup your XDP incorrectly", len(msg.RawSample)))
					return
				}

				flowCopy := make([]byte, len(msg.RawSample))
				copy(flowCopy, msg.RawSample)

				dataChan <- flowCopy
			}
		}
	}
}

func (mr *MapReader) CloseMap() error {
	if mr.ebpfMap != nil {
		return mr.ebpfMap.Close()
	}
	return nil
}
