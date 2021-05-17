package constants

import "0chain.net/core/common"

const (
	ALLOCATION_CONTEXT_KEY common.ContextKey = "allocation"
	CLIENT_CONTEXT_KEY     common.ContextKey = "client"
	CLIENT_KEY_CONTEXT_KEY common.ContextKey = "client_key"

	// CLIENT_SIGNATURE_HEADER_KEY represents key for context value passed with common.ClientSignatureHeader request header.
	CLIENT_SIGNATURE_HEADER_KEY common.ContextKey = "signature"
)
