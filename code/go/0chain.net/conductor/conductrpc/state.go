package conductrpc

import (
	"github.com/0chain/blobber/code/go/0chain.net/conductor/config"
)

//
// state (long polling)
//

type (
	// BlobberList
	BlobberList struct {
		SendWrongData     bool   `json:"send_wrong_data" yaml:"send_wrong_data" mapstructure:"send_wrong_data"`
		SendWrongMetadata bool   `json:"send_wrong_metadata" yaml:"send_wrong_metadata" mapstructure:"send_wrong_metadata"`
		NotRespond        bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial       string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}
	// BlobberDownload
	BlobberDownload struct {
		NotRespond  bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}
	// BlobberUpload
	BlobberUpload struct {
		NotRespond  bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}
	// BlobberDelete
	BlobberDelete struct {
		NotRespond  bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}
)

// State is current node state.
type State struct {
	// Nodes maps NodeID -> NodeName.
	Nodes map[NodeID]NodeName

	IsMonitor  bool // send monitor events (round, phase, etc)
	IsLock     bool // node locked
	IsRevealed bool // revealed shares
	// Byzantine state. Below, if a value is nil, then node behaves as usual
	// for it.
	//
	// Byzantine blockchain
	VRFS                        *config.Bad
	RoundTimeout                *config.Bad
	CompetingBlock              *config.Bad
	SignOnlyCompetingBlocks     *config.Bad
	DoubleSpendTransaction      *config.Bad
	WrongBlockSignHash          *config.Bad
	WrongBlockSignKey           *config.Bad
	WrongBlockHash              *config.Bad
	VerificationTicketGroup     *config.Bad
	WrongVerificationTicketHash *config.Bad
	WrongVerificationTicketKey  *config.Bad
	WrongNotarizedBlockHash     *config.Bad
	WrongNotarizedBlockKey      *config.Bad
	NotarizeOnlyCompetingBlock  *config.Bad
	NotarizedBlock              *config.Bad
	// Byzantine blockchain sharders
	FinalizedBlock    *config.Bad
	MagicBlock        *config.Bad
	VerifyTransaction *config.Bad
	// Byzantine View Change
	MPK        *config.Bad
	Shares     *config.Bad
	Signatures *config.Bad
	Publish    *config.Bad

	// Blobbers related states
	StorageTree     *config.Bad // blobber sends bad files/tree responses
	ValidatorProof  *config.Bad // blobber sends invalid proof to validators
	Challenges      *config.Bad // blobber ignores challenges
	BlobberList     BlobberList
	BlobberDownload BlobberDownload
	BlobberUpload   BlobberUpload
	BlobberDelete   BlobberDelete
}

// Name returns NodeName by given NodeID.
func (s *State) Name(id NodeID) NodeName {
	return s.Nodes[id] // id -> name (or empty string)
}

func (s *State) copy() (cp *State) { //nolint:unused,deadcode // might be used later?
	cp = new(State)
	(*cp) = (*s)
	return
}

func (s *State) send(poll chan *State) { //nolint:unused,deadcode // might be used later?
	go func(state *State) {
		poll <- state
	}(s.copy())
}

type IsGoodBader interface {
	IsGood(state config.Namer, id string) bool
	IsBad(state config.Namer, id string) bool
}

type IsByer interface {
	IsBy(state config.Namer, id string) bool
}
