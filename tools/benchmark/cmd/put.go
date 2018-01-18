// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/cheggaaa/pb.v2"
)

// putCmd represents the put command
var putCmd = &cobra.Command{
	Use:   "put [path](default is /)",
	Short: "Benchmark put",

	Run: putFunc,
}

var (
	putTotal int
	keySize  int
	valSize  int
)

func init() {
	RootCmd.AddCommand(putCmd)
	putCmd.Flags().IntVar(&putTotal, "total", 10000, "Total number of put requests")
	putCmd.Flags().IntVar(&keySize, "key-size", 8, "Key size of put request")
	putCmd.Flags().IntVar(&valSize, "val-size", 8, "Value size of put request")
}

func putFunc(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}
	p := "/"

	if len(args) == 1 {
		p = args[0]
	}

	results = make(chan result)
	requests := make(chan *http.Request, totalClients)
	bar = &pb.ProgressBar{}
	bar.SetTotal(int64(putTotal))

	clients := makeHttpClients(totalClients)

	bar.Start()

	for i := range clients {
		wg.Add(1)
		go clientDo(clients[i], requests)
	}

	pdoneC := printReport(results)

	go func() {
		for i := 0; i < putTotal; i++ {
			data, _ := json.Marshal(generatePutData())
			reader := strings.NewReader(string(data))
			requests <- makeManageRequest("PUT", path.Join("/v1/data", p), reader)
		}
		close(requests)
	}()

	wg.Wait()

	bar.Finish()

	close(results)
	<-pdoneC
}

func generatePutData() map[string]interface{} {
	key := RandomString(keySize)
	value := RandomString(valSize)
	folder := key
	if len(key) > 2 {
		folder = key[:2]
	}
	data := map[string]interface{}{
		folder: map[string]string{
			key: value,
		},
	}
	return data
}
