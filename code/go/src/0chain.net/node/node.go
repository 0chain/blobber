package node

import (
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"0chain.net/common"
	"0chain.net/config"
	"0chain.net/encryption"
)

var nodes = make(map[string]*Node)

/*RegisterNode - register a node to a global registery
* We need to keep track of a global register of nodes. This is required to ensure we can verify a signed request
* coming from a node
 */
func RegisterNode(node *Node) {
	nodes[node.GetKey()] = node
}

/*DeregisterNode - deregisters a node */
func DeregisterNode(nodeID string) {
	delete(nodes, nodeID)
}

/*GetNode - get the node from the registery */
func GetNode(nodeID string) *Node {
	return nodes[nodeID]
}

var (
	NodeStatusInactive = 0
	NodeStatusActive   = 1
)

var (
	NodeTypeMiner   = 0
	NodeTypeSharder = 1
	NodeTypeBlobber = 2
)

var NodeTypeNames []*common.Lookup = common.CreateLookups("m", "Miner", "s", "Sharder", "b", "Blobber")

/*Node - a struct holding the node information */
type Node struct {
	ID             string
	Version        string
	CreationDate   common.Timestamp
	PublicKey      string
	PublicKeyBytes encryption.HashBytes
	Host           string
	Port           int
	Type           int
}

/*Provider - create a node object */
func Provider() *Node {
	node := &Node{}

	return node
}

/*Equals - if two nodes are equal. Only check by id, we don't accept configuration from anyone */
func (n *Node) Equals(n2 *Node) bool {
	if common.IsEqual(n.GetKey(), n2.GetKey()) {
		return true
	}
	if n.Port == n2.Port && n.Host == n2.Host {
		return true
	}
	return false
}

/*Print - print node's info that is consumable by Read */
func (n *Node) Print(w io.Writer) {
	fmt.Fprintf(w, "%v,%v,%v,%v,%v\n", n.GetNodeType(), n.Host, n.Port, n.GetKey(), n.PublicKey)
}

/*Read - read a node config line and create the node */
func Read(line string) (*Node, error) {
	node := Provider()
	fields := strings.Split(line, ",")
	if len(fields) != 5 {
		return nil, common.NewError("invalid_num_fields", fmt.Sprintf("invalid number of fields [%v]", line))
	}
	switch fields[0] {
	case "m":
		node.Type = NodeTypeMiner
	case "s":
		node.Type = NodeTypeSharder
	case "b":
		node.Type = NodeTypeBlobber
	default:
		return nil, common.NewError("unknown_node_type", fmt.Sprintf("Unkown node type %v", fields[0]))
	}
	node.Host = fields[1]
	if node.Host == "" {
		if node.Port != config.Configuration.Port {
			node.Host = config.Configuration.Host
		} else {
			panic(fmt.Sprintf("invalid node setup for %v\n", node.GetKey()))
		}
	}

	port, err := strconv.ParseInt(fields[2], 10, 32)
	if err != nil {
		return nil, err
	}
	node.Port = int(port)
	node.ID = fields[3]
	node.PublicKey = fields[4]
	node.SetPublicKey(node.PublicKey)
	hash := encryption.Hash(node.PublicKeyBytes)
	if node.ID != hash {
		return nil, common.NewError("invalid_client_id", fmt.Sprintf("public key: %v, client_id: %v, hash: %v\n", node.PublicKey, node.ID, hash))
	}
	node.ComputeProperties()
	if Self.PublicKey == node.PublicKey {
		Self.Node = node
	}
	return node, nil
}

/*ComputeProperties - implement interface */
func (n *Node) ComputeProperties() {
	n.computePublicKeyBytes(n.PublicKey)
}

func (n *Node) computePublicKeyBytes(key string) {
	b, _ := hex.DecodeString(key)
	if len(b) > len(n.PublicKeyBytes) {
		b = b[len(b)-encryption.HASH_LENGTH:]
	}
	copy(n.PublicKeyBytes[encryption.HASH_LENGTH-len(b):], b)
}

/*SetPublicKey - set the public key */
func (n *Node) SetPublicKey(key string) {
	n.PublicKey = key
	n.computePublicKeyBytes(key)
	n.ID = encryption.Hash(n.PublicKeyBytes)
}

/*GetURLBase - get the end point base */
func (n *Node) GetURLBase() string {
	host := n.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%v:%v", host, n.Port)
}

/*GetNodeType - as a string */
func (n *Node) GetNodeType() string {
	return NodeTypeNames[n.Type].Code
}

/*GetNodeTypeName - as a string */
func (n *Node) GetNodeTypeName() string {
	return NodeTypeNames[n.Type].Value
}

/*GetKey - Get Key of the node */
func (n *Node) GetKey() string {
	return n.ID
}
