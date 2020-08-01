package utils

import (
	"0chain.net/core/node"

	crpc "0chain.net/conductor/conductrpc"
)

// Split nodes list by given IsGoodBader.
func Split(s *crpc.State, igb crpc.IsGoodBader, nodes []*node.Node) (
	good, bad []*node.Node) {

	for _, n := range nodes {
		if igb.IsBad(s, n.GetKey()) {
			bad = append(bad, n)
		} else if igb.IsGood(s, n.GetKey()) {
			good = append(good, n)
		}
	}
	return
}

// Filter return IsBy nodes only.
func Filter(s *crpc.State, ib crpc.IsByer, nodes []*node.Node) (
	rest []*node.Node) {

	for _, n := range nodes {
		if ib.IsBy(s, n.GetKey()) {
			rest = append(rest, n)
		}
	}
	return
}
