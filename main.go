// Copyright (c) 2024 Carsen Klock under MIT License
// mactop is a simple terminal based Apple Silicon power monitor written in Go Lang!
// github.com/context-labs/mactop
package main

import (
	"github.com/context-labs/mactop/v2/cmd"
	"github.com/sirupsen/logrus"
)

func main() {
	var err = cmd.Execute()
	if err != nil {
		logrus.Fatalf("failed to run mactop: %v", err)
	}
}
