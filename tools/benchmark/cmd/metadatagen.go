// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package cmd

// Generate metadata for benchmark,
// The metadata format for this benchmark is the metadata format for simulating qingcloud appcenter.
import "fmt"

const (
	ClusterSize    = 20
	NodePerCluster = 20
	EnvSize        = 5
)

func genMetadata() map[string]interface{} {
	data := map[string]interface{}{}
	data["clusters"] = genClusters()
	return data
}

func genMappings(data map[string]interface{}) map[string]interface{} {
	clusters := data["clusters"].(map[string]interface{})
	mappings := map[string]interface{}{}
	for cid, cluster := range clusters {
		clusterMap := cluster.(map[string]interface{})
		hosts := clusterMap["hosts"].(map[string]interface{})
		for hid, host := range hosts {
			hostMap := host.(map[string]interface{})
			ip := hostMap["ip"].(string)
			mappings[ip] = genMapping(cid, hid)
		}
	}
	return mappings
}

func genMapping(clusterID string, hostID string) map[string]interface{} {
	mapping := map[string]interface{}{
		"cluster": fmt.Sprintf("/clusters/%s/cluster", clusterID),
		"cmd":     fmt.Sprintf("/clusters/%s/cmd/%s", clusterID, hostID),
		"env":     fmt.Sprintf("/clusters/%s/env", clusterID),
		"host":    fmt.Sprintf("/clusters/%s/hosts/%s", clusterID, hostID),
		"hosts":   fmt.Sprintf("/clusters/%s/hosts", clusterID),
	}
	return mapping
}

func genClusters() map[string]interface{} {
	clusters := map[string]interface{}{}
	for i := 0; i < ClusterSize; i++ {
		clusterID := genClusterID(clusters)
		cluster := map[string]interface{}{}
		clusters[clusterID] = cluster
		cluster["cluster"] = genCluster(clusterID)
		cmdData := map[string]interface{}{}
		cluster["cmd"] = cmdData
		hosts := genHosts(i)
		cluster["hosts"] = hosts
		for host, _ := range hosts {
			cmdData[host] = genCmd()
		}
		cluster["env"] = genEnv()
	}
	return clusters
}

func genClusterID(clusters map[string]interface{}) string {
	clusterID := fmt.Sprintf("cl-%s", RandomString(8))
	for _, ok := clusters[clusterID]; ok; {
		clusterID = fmt.Sprintf("cl-%s", RandomString(8))
	}
	return clusterID
}

func genCluster(clusterID string) map[string]interface{} {
	cluster := map[string]interface{}{}
	cluster["app_id"] = fmt.Sprintf("app-%s", RandomString(8))
	cluster["cluster_id"] = clusterID
	cluster["global_uuid"] = RandomString(8)
	cluster["vxnet"] = fmt.Sprintf("vxnet-%s", RandomString(7))
	cluster["zone"] = "pek3a"
	return cluster
}

func genHosts(clusterIdx int) map[string]interface{} {
	hosts := map[string]interface{}{}
	for i := 0; i < NodePerCluster; i++ {
		hostID := genHostID(hosts)
		hosts[hostID] = genHost(hostID, clusterIdx, i)
	}
	return hosts
}

func genHostID(hosts map[string]interface{}) string {
	hostID := fmt.Sprintf("i-%s", RandomString(8))
	for _, ok := hosts[hostID]; ok; {
		hostID = fmt.Sprintf("i-%s", RandomString(8))
	}
	return hostID
}

func genHost(hostID string, clusterIdx, idx int) map[string]interface{} {
	host := map[string]interface{}{}
	host["cpu"] = "2"
	host["gid"] = "2"
	host["gsid"] = "556818536"
	host["instance_id"] = hostID
	host["ip"] = fmt.Sprintf("192.168.%v.%v", clusterIdx, idx)
	host["mac"] = "52:54:8c:13:16:d9"
	host["memory"] = "2048"
	host["node_id"] = fmt.Sprintf("cln-%s", RandomString(8))
	host["physical_machine"] = RandomString(32)
	host["sid"] = fmt.Sprintf("%v", idx)
	return host
}

func genCmd() map[string]interface{} {
	cmdData := map[string]interface{}{
		"cmd":     "systemctl start xservice",
		"id":      RandomString(32),
		"timeout": "86400",
	}
	return cmdData
}

func genEnv() map[string]interface{} {
	env := map[string]interface{}{}
	for i := 0; i < EnvSize; i++ {
		env[RandomString(8)] = RandomString(8)
	}
	return env
}
