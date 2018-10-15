package node

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"

	"0chain.net/common"
	. "0chain.net/logging"
)

var ErrNodeNotFound = common.NewError("node_not_found", "Requested node is not found")

/*Pool - a pool of nodes used for the same purpose */
type Pool struct {
	//Mutex &sync.Mutex{}
	Type     int
	Nodes    []*Node
	NodesMap map[string]*Node
}

/*NewPool - create a new node pool of given type */
func NewPool(Type int) *Pool {
	np := Pool{Type: Type}
	np.NodesMap = make(map[string]*Node)
	return &np
}

/*Size - size of the pool without regards to the node status */
func (np *Pool) Size() int {
	return len(np.Nodes)
}

/*NewNode - read a node config line and create the node */
func NewNode(nc map[interface{}]interface{}) (*Node, error) {
	node := Provider()
	node.Type = nc["type"].(int)
	node.Host = nc["public_ip"].(string)

	node.Port = nc["port"].(int)
	node.ID = nc["id"].(string)
	node.PublicKey = nc["public_key"].(string)
	node.Description = nc["description"].(string)
	return node, nil
}

/*AddNodes - add nodes to the node pool */
func (np *Pool) AddNodes(nodes []interface{}) {
	for _, nci := range nodes {
		nc, ok := nci.(map[interface{}]interface{})
		if !ok {
			continue
		}
		nc["type"] = np.Type
		nd, err := NewNode(nc)
		if err != nil {
			panic(err)
		}
		np.AddNode(nd, false) //We will computeArray after we add all the nodes
	}
	np.computeNodesArray() //Add nodes to nodes array.
}

/*AddNode - add a nodes to the pool */
func (np *Pool) AddNode(node *Node, doCompute bool) {
	if np.Type != node.Type {
		Logger.Info("did not add node to the nodemap. Node Type = " + fmt.Sprintf("%v", node.Type))
		return
	}
	var nodeID = common.ToKey(node.GetKey())
	np.NodesMap[nodeID] = node

	if doCompute {
		np.computeNodesArray()
	} //else we do not want to compute as it does array allocation
}

/*GetNode - given node id, get the node object or nil */
func (np *Pool) GetNode(id string) *Node {
	node, ok := np.NodesMap[id]
	if !ok {
		return nil
	}
	return node
}

/*RemoveNode - Remove a node from the pool */
func (np *Pool) RemoveNode(nodeID string) {
	if _, ok := np.NodesMap[nodeID]; !ok {
		return
	}
	delete(np.NodesMap, nodeID)
	np.computeNodesArray()
}

var none = make([]*Node, 0)

func (np *Pool) shuffleNodes() []*Node {
	size := np.Size()
	if size == 0 {
		return none
	}
	shuffled := make([]*Node, size)
	perm := rand.Perm(size)
	for i, v := range perm {
		shuffled[v] = np.Nodes[i]
	}
	return shuffled
}

func (np *Pool) computeNodesArray() {
	// TODO: Do we need to use Mutex while doing this?
	var array = make([]*Node, 0, len(np.NodesMap))
	for _, v := range np.NodesMap {
		array = append(array, v)
	}
	np.Nodes = array
}

/*GetRandomNodes - get a random set of nodes from the pool
* Doesn't consider active/inactive status
 */
func (np *Pool) GetRandomNodes(num int) []*Node {
	var size = np.Size()
	if num > size {
		num = size
	}
	nodes := np.shuffleNodes()
	return nodes[:num]
}

/*GetRandomNode - get a random node from the pool
* Doesn't consider active/inactive status
 */
func (np *Pool) GetRandomNode() *Node {
	nodes := np.GetRandomNodes(1)
	return nodes[0]
}

/*Print - print this pool. This will be used for http response and Read method should be able to consume it*/
func (np *Pool) Print(w io.Writer) {
	nodes := np.shuffleNodes()
	for _, node := range nodes {
		node.Print(w)
	}
}

/*ReadNodes - read the pool information */
func ReadNodes(r io.Reader, minerPool *Pool, sharderPool *Pool, blobberPool *Pool) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		node, err := Read(line)
		if err != nil {
			panic(err)
		}
		/*
			if Self != nil && node.Equals(Self.Node) {
				continue
			}*/
		switch node.Type {
		case NodeTypeMiner:
			minerPool.AddNode(node, false)
		case NodeTypeSharder:
			sharderPool.AddNode(node, false)
		case NodeTypeBlobber:
			blobberPool.AddNode(node, false)
		default:
			panic(fmt.Sprintf("unkown node type %v:%v\n", node.GetKey(), node.Type))
		}
	}
}

func (np *Pool) ComputeProperties() {
	np.computeNodesArray()
	for _, node := range np.Nodes {
		RegisterNode(node)
	}
}
