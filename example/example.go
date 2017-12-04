package main

import (
	"time"

	"github.com/gogap/logrus_mate"
	"github.com/sirupsen/logrus"

	_ "github.com/drdaeman/logdna-logrus"
)

func main() {
	mate, err := logrus_mate.NewLogrusMate(
		logrus_mate.ConfigFile("example.conf"),
	)
	if err != nil {
		logrus.WithError(err).Error("Logrus Mate initialization failed")
		return
	}

	logger := logrus.New()
	if err = mate.Hijack(logger, "example"); err != nil {
		logrus.WithError(err).Error("Failed to configure example logger")
		return
	}

	defer logrus.Exit(0) // Important: hook needs to flush pending data
	logger.WithField(
		"Now", time.Now().Format(time.RFC3339),
	).Info("Hello, world!")
}
