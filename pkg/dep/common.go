package dep

import "github.com/Jguer/yay/v12/pkg/dep/topo"

func validateAndSetNodeInfo(graph *topo.Graph[string, *InstallInfo],
	node string, nodeInfo *topo.NodeInfo[*InstallInfo],
) {
	info := graph.GetNodeInfo(node)
	if info != nil && info.Value != nil {
		if info.Value.Reason < nodeInfo.Value.Reason {
			return // refuse to downgrade reason
		}

		if info.Value.Upgrade {
			return // refuse to overwrite an upgrade
		}
	}

	graph.SetNodeInfo(node, nodeInfo)
}
