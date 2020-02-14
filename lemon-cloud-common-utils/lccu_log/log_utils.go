package lccu_log

import (
	"github.com/sirupsen/logrus"
)

func Info(args ...interface{}) {
	logrus.Info(args)
}

func Infof(format string, args ...interface{}) {
	logrus.Infof(format, args)
}

func Infoln(args ...interface{}) {
	logrus.Infoln(args)
}

func Warn(args ...interface{}) {
	logrus.Warn(args)
}

func Warnf(format string, args ...interface{}) {
	logrus.Warnf(format, args)
}

func Warnln(args ...interface{}) {
	logrus.Warnln(args)
}

func Error(args ...interface{}) {
	logrus.Error(args)
}

func Errorf(format string, args ...interface{}) {
	logrus.Errorf(format, args)
}

func Errorln(args ...interface{}) {
	logrus.Errorln(args)
}

func Debug(args ...interface{}) {
	logrus.Debug(args)
}

func Debugf(format string, args ...interface{}) {
	logrus.Debugf(format, args)
}

func Debugln(args ...interface{}) {
	logrus.Debugln(args)
}
