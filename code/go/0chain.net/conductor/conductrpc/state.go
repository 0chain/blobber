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
		ReturnError       bool   `json:"return_error" yaml:"return_error" mapstructure:"return_error"`
		SendWrongData     bool   `json:"send_wrong_data" yaml:"send_wrong_data" mapstructure:"send_wrong_data"`
		SendWrongMetadata bool   `json:"send_wrong_metadata" yaml:"send_wrong_metadata" mapstructure:"send_wrong_metadata"`
		NotRespond        bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial       string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}
	// BlobberDownload
	BlobberDownload struct {
		ReturnError bool   `json:"return_error" yaml:"return_error" mapstructure:"return_error"`
		NotRespond  bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}
	// BlobberUpload
	BlobberUpload struct {
		ReturnError bool   `json:"return_error" yaml:"return_error" mapstructure:"return_error"`
		NotRespond  bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}
	// BlobberDelete
	BlobberDelete struct {
		ReturnError bool   `json:"return_error" yaml:"return_error" mapstructure:"return_error"`
		NotRespond  bool   `json:"not_respond" yaml:"not_respond" mapstructure:"not_respond"`
		Adversarial string `json:"adversarial" yaml:"adversarial" mapstructure:"adversarial"`
	}

	// AdversarialValidator
	AdversarialValidator struct {
		ID                 string `json:"id" yaml:"id" mapstructure:"id"`
		FailValidChallenge bool   `json:"fail_valid_challenge" yaml:"fail_valid_challenge" mapstructure:"fail_valid_challenge"`
		DenialOfService    bool   `json:"denial_of_service" yaml:"denial_of_service" mapstructure:"denial_of_service"`
		PassAllChallenges  bool   `json:"pass_all_challenges" yaml:"pass_all_challenges" mapstructure:"pass_all_challenges"`
	}
)

// State is current node state.
type State struct {
	// Nodes maps NodeID -> NodeName.
	Nodes map[NodeID]NodeName

	IsMonitor bool // send monitor events (round, phase, etc)
	IsLock    bool // node locked

	// Blobbers related states
	StorageTree          *config.Bad // blobber sends bad files/tree responses
	ValidatorProof       *config.Bad // blobber sends invalid proof to validators
	Challenges           *config.Bad // blobber ignores challenges
	BlobberList          BlobberList
	BlobberDownload      BlobberDownload
	BlobberUpload        BlobberUpload
	BlobberDelete        BlobberDelete
	AdversarialValidator AdversarialValidator
	NotifyOnValidationTicketGeneration bool
	StopWMCommit         *bool
	FailRenameCommit     []string
	FailUploadCommit     []string
	GetFileMetaRoot      bool
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
