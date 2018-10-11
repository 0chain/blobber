package transaction

import (
	"0chain.net/common"
)

const TRANSACTION_VERION 	=	"1.0"

//Transaction entity that encapsulates the transaction related data and meta data
type Transaction struct {
	Hash 				string				`json:"hash,omitempty"`
	Version 			string 				`json:"version,omitempty"`
	ClientID  			string 				`json:"client_id,omitempty"`
	PublicKey 			string        		`json:"public_key,omitempty"`
	ToClientID 			string    		 	`json:"to_client_id,omitempty"`
	ChainID 			string    			`json:"chain_id,omitempty"`
	Value 				int64 				`json:"transaction_value,omitempty"`
	Data 				string	    		`json:"transaction_data",omitempty"`
	TxType 				int 				`json:"transaction_type",omitempty"`
	CreationDate 		common.Timestamp    `json:"creation_date",omitempty"`
	Signature 			string 				`json:"signature",omitempty"`
}
