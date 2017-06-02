package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/cheggaaa/pb.v2"
	"net/http"
	"os"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [path](default is /)",
	Short: "Benchmark get",

	Run: getFunc,
}

var (
	getTotal int
)

func init() {
	RootCmd.AddCommand(getCmd)
	getCmd.Flags().IntVar(&getTotal, "total", 10000, "Total number of get requests")
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
