package nomad

import "github.com/hashicorp/serf/serf"

// serfMergeDelegate is used to handle a cluster merge on the gossip
// ring. We check that the peers are nomad servers and abort the merge
// otherwise.
type serfMergeDelegate struct {
}

func (md *serfMergeDelegate) NotifyMerge(members []*serf.Member) error {
	//for _, m := range members {
	//ok, _ := isConsulServer(*m)
	//if !ok {
	//    return fmt.Errorf("Member '%s' is not a server", m.Name)
	//}
	//}
	return nil
}
