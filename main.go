package main

import (
	"github.com/schwarzm/envy/pkg/cmd"
)

var VERSION string

func main() {
	cmd.InitFlags()
	cmd.Execute()
}
