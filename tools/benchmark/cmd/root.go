// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package cmd

import (
	"sync"

	"github.com/spf13/cobra"
	"gopkg.in/cheggaaa/pb.v2"
)

// This represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "A benchmark tool for metad",
}

var (
	endpoints      []string
	manageEndpoint string
	totalClients   uint
	sample         bool

	bar     *pb.ProgressBar
	results chan result
	wg      sync.WaitGroup

	xff string
)

func init() {
	RootCmd.PersistentFlags().StringSliceVar(&endpoints, "endpoints", []string{"http://127.0.0.1"}, "metad api address")
	RootCmd.PersistentFlags().StringVar(&manageEndpoint, "manage_endpoint", "http://127.0.0.1:9611", "metad manage api address")
	RootCmd.PersistentFlags().UintVar(&totalClients, "clients", 1, "Total number of http clients")

	RootCmd.PersistentFlags().BoolVar(&sample, "sample", false, "'true' to sample requests for every second")

	RootCmd.PersistentFlags().StringVar(&xff, "xff", "", "set http X-Forwarded-For header.")
}
