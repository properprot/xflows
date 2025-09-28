package outputs

import (
	"github.com/properprot/xflows/pkg/sflow"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type Output interface {
	Send(sample *sflow.SFlowSample) error
	Close() error
}
