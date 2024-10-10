package main

import (
    "os"

    "github.com/sirupsen/logrus"
)

func main() {
    // Initialize Logrus
    logrus.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
    })
    logrus.SetOutput(os.Stdout)
    logrus.SetLevel(logrus.InfoLevel)

    logrus.Info("Starting Development Environment Manager...")
    Execute() // Executes the root command defined in cmd.go
}