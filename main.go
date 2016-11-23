package main

import (
	"flag"
	"fmt"
	"github.com/yunify/metad/log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
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
		fmt.Printf("Metad Version: %s\n", VERSION)
		fmt.Printf("Git Version: %s\n", GIT_VERSION)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if pprof {
		fmt.Printf("Start pprof, 127.0.0.1:6060\n")
		go func() {
			log.Fatal("%v", http.ListenAndServe("127.0.0.1:6060", nil))
		}()
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
