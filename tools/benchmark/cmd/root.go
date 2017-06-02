// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	cpuProfPath string
	memProfPath string

	xff string
)

func init() {
	RootCmd.PersistentFlags().StringSliceVar(&endpoints, "endpoints", []string{"http://127.0.0.1"}, "metad api address")
	RootCmd.PersistentFlags().StringVar(&manageEndpoint, "manage_endpoint", "http://127.0.0.1:9611", "metad manage api address")
	RootCmd.PersistentFlags().UintVar(&totalClients, "clients", 1, "Total number of http clients")

	RootCmd.PersistentFlags().BoolVar(&sample, "sample", false, "'true' to sample requests for every second")

	RootCmd.PersistentFlags().StringVar(&xff, "xff", "", "set http X-Forwarded-For header.")
}
