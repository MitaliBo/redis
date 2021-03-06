/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package configure_cluster

import (
	"strings"

	"github.com/appscode/go/log"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"kmodules.xyz/client-go/tools/exec"
)

func (c Config) createCluster(pod *core.Pod, addrs ...string) error {
	options := []func(options *exec.Options){
		exec.Input("yes"),
		exec.Command(c.ClusterCreateCmd(0, addrs...)...),
	}
	_, err := exec.ExecIntoPod(c.RestConfig, pod, options...)
	if err != nil {
		return errors.Wrapf(err, "Failed to create cluster using (%v)", addrs)
	}

	return nil
}

func (c Config) addNode(pod *core.Pod, newAddr, existingAddr, masterId string) error {
	var err error

	if masterId == "" {
		if _, err = exec.ExecIntoPod(c.RestConfig, pod, exec.Command(c.AddNodeAsMasterCmd(newAddr, existingAddr)...)); err != nil {
			return errors.Wrapf(err, "Failed to add %q as a master", newAddr)
		}
	} else {
		if _, err = exec.ExecIntoPod(c.RestConfig, pod, exec.Command(c.AddNodeAsSlaveCmd(newAddr, existingAddr, masterId)...)); err != nil {
			return errors.Wrapf(err, "Failed to add %q as a slave of master with id %q", newAddr, masterId)
		}
	}

	return nil
}

func (c Config) deleteNode(pod *core.Pod, existingAddr, deletingNodeID string) error {
	_, err := exec.ExecIntoPod(c.RestConfig, pod, exec.Command(c.DeleteNodeCmd(existingAddr, deletingNodeID)...))
	if err != nil {
		return errors.Wrapf(err, "Failed to delete node with ID %q", deletingNodeID)
	}

	return nil
}

func (c Config) ping(pod *core.Pod, ip string) (string, error) {
	pong, err := exec.ExecIntoPod(c.RestConfig, pod, exec.Command(PingCmd(ip)...))
	if err != nil {
		return "", errors.Wrapf(err, "Failed to ping %q", pod.Status.PodIP)
	}

	return strings.TrimSpace(pong), nil
}

func (c Config) getClusterNodes(pod *core.Pod, ip string) (string, error) {
	out, err := exec.ExecIntoPod(c.RestConfig, pod, exec.Command(ClusterNodesCmd(ip)...))
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get cluster nodes from %q", ip)
	}

	return strings.TrimSpace(out), nil
}

func (c Config) clusterMeet(pod *core.Pod, senderIP, receiverIP, receiverPort string) error {
	_, err := exec.ExecIntoPod(c.RestConfig, pod, exec.Command(ClusterMeetCmd(senderIP, receiverIP, receiverPort)...))
	if err != nil {
		return errors.Wrapf(err, "Failed to meet node %q with node %q", senderIP, receiverIP)
	}

	return nil
}

func (c Config) clusterReset(pod *core.Pod, ip, resetType string) error {
	_, err := exec.ExecIntoPod(c.RestConfig, pod, exec.Command(ClusterResetCmd(ip, resetType)...))
	if err != nil {
		return errors.Wrapf(err, "Failed to reset node %q", ip)
	}

	return nil
}

func (c Config) clusterFailover(pod *core.Pod, ip string) error {
	_, err := exec.ExecIntoPod(c.RestConfig, pod, exec.Command(ClusterFailoverCmd(ip)...))
	if err != nil {
		return errors.Wrapf(err, "Failed to failover node %q", ip)
	}

	return nil
}

func (c Config) clusterReplicate(pod *core.Pod, receivingNodeIP, masterNodeID string) error {
	_, err := exec.ExecIntoPod(c.RestConfig, pod, exec.Command(ClusterReplicateCmd(receivingNodeIP, masterNodeID)...))
	if err != nil {
		return errors.Wrapf(err, "Failed to replicate node %q of node with ID %s",
			receivingNodeIP, masterNodeID)
	}

	return nil
}

func (c Config) reshard(pod *core.Pod, nodes [][]RedisNode, src, dst, requstedSlotsCount int) error {
	log.Infof("Resharding %d slots from %q to %q...", requstedSlotsCount, nodes[src][0].IP, nodes[dst][0].IP)

	var (
		need int
		err  error
	)

	need = requstedSlotsCount

	for i := range nodes[src][0].SlotStart {
		if need <= 0 {
			break
		}

		start := nodes[src][0].SlotStart[i]
		end := nodes[src][0].SlotEnd[i]
		if end-start+1 > need {
			end = start + need - 1
		}
		cmd := c.ReshardCmd(nodes[src][0].IP, nodes[src][0].ID, nodes[dst][0].IP, nodes[dst][0].ID, start, end)

		_, err = exec.ExecIntoPod(c.RestConfig, pod, exec.Command(cmd...))
		if err != nil {
			return errors.Wrapf(err, "Failed to reshard %d slots from %q to %q",
				requstedSlotsCount, nodes[src][0].IP, nodes[dst][0].IP)
		}

		need -= (end - start + 1)
	}

	return nil
}
