package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	xflows "github.com/properprot/xflows/pkg"
	"github.com/sirupsen/logrus"
)

type XFlowsXDP struct {
	SFlows *ebpf.Map     `ebpf:"sflows"`
	Prog   *ebpf.Program `ebpf:"xdp_main"`
}

func main() {
	logger := logrus.New()

	exporter, err := xflows.NewExporter("config.yaml")
	if err != nil {
		logger.WithError(err).Fatal("Failed to create XFlows exporter")
	}

	spec, err := ebpf.LoadCollectionSpec("xflows.o")
	if err != nil {
		logger.WithError(err).Fatal("Failed to load eBPF collection")
	}

	var program XFlowsXDP
	if err := spec.LoadAndAssign(&program, nil); err != nil {
		logger.WithError(err).Fatal("Failed to load eBPF collection")
	}

	ifIndex, err := net.InterfaceByName("eth0")
	if err != nil {
		logger.WithError(err).Fatal("Failed to get interface")
	}

	xdplink, xdperr := link.AttachXDP(link.XDPOptions{
		Program:   program.Prog,
		Interface: ifIndex.Index,
		Flags:     link.XDPDriverMode, // Can be commented for XDP Generic (but you shouldn't)
	})
	if xdperr != nil {
		logger.WithError(xdperr).Fatal("Failed to attach XDP")
	}

	defer xdplink.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	logger.Info("Shutting down")
	exporter.Stop()
}
