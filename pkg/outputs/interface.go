package outputs

import (
	"github.com/properprot/xflows/pkg/sflow"
	"github.com/sirupsen/logrus"
)

var log = &logrus.Logger{}

type Output interface {
	Send(sample *sflow.SFlowSample) error
	Close() error
}
