package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log = logrus.New()

func init() {
	Log.Out = os.Stdout
}

func SetLog(logger *logrus.Logger) {
	Log = logger
}
