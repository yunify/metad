// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/cheggaaa/pb.v2"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [path](default is /)",
	Short: "Benchmark get",

	Run: getFunc,
}

var (
	getTotal         int
	fillSimulateData bool
)

func init() {
	RootCmd.AddCommand(getCmd)
	getCmd.Flags().IntVar(&getTotal, "total", 10000, "Total number of get requests")
	getCmd.Flags().BoolVar(&fillSimulateData, "fill_simulate_data", false, "Fill simulate metadata before get bentchmark.")
}

func getFunc(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}
	path := "/"

	if len(args) == 1 {
		path = args[0]
	}

	if fillSimulateData {
		data := genMetadata()
		mapping := genMappings(data)
		fillMetadata(data, mapping)
		fmt.Println("fill metadata finish.")
	}

	results = make(chan result)
	requests := make(chan *http.Request, totalClients)
	bar = &pb.ProgressBar{}
	bar.SetTotal(int64(getTotal))

	clients := makeHttpClients(totalClients)

	bar.Start()

	for i := range clients {
		wg.Add(1)
		go clientDo(clients[i], requests)
	}

	pdoneC := printReport(results)

	go func() {
		for i := 0; i < getTotal; i++ {
			requests <- makeMetaRequest(path)
		}
		close(requests)
	}()

	wg.Wait()

	bar.Finish()

	close(results)
	<-pdoneC
}
