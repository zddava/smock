package main

import (
	"flag"
	"fmt"

	"github.com/zddava/smock/build"
	"github.com/zddava/smock/conf"
)

var (
	version = flag.Bool("v", false, "version")
)

func main() {
	flag.Parse()
	if *version {
		fmt.Println(build.ToString())
		return
	}

	conf.ParseAndRun()

	select {}
}
