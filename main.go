package main

import (
	"flag"
	"fmt"
	"github.com/yunify/metad/log"
	"os"
	"time"
)

func main() {

	defer func() {
		if r := recover(); r != nil {
			log.Error("Main Recover: %v, try restart.", r)
			time.Sleep(time.Duration(1000) * time.Millisecond)
			main()
		}
	}()

	flag.Parse()

	if printVersion {
		fmt.Printf("%s\n", VERSION)
		os.Exit(0)
	}
	var config *Config
	var err error
	if config, err = initConfig(); err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}

	log.Info("Starting metad %s", VERSION)
	metad, err = New(config)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}

	metad.Init()
	metad.Serve()
}
