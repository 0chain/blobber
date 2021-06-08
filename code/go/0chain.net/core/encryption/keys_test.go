package encryption

import (
	"fmt"
	"testing"
	"github.com/herumi/bls-go-binary/bls"
	"github.com/stretchr/testify/require"
)

func TestMiraclToHerumiPK(t *testing.T) {
	miraclpk1 := `0418a02c6bd223ae0dfda1d2f9a3c81726ab436ce5e9d17c531ff0a385a13a0b491bdfed3a85690775ee35c61678957aaba7b1a1899438829f1dc94248d87ed36817f6dfafec19bfa87bf791a4d694f43fec227ae6f5a867490e30328cac05eaff039ac7dfc3364e851ebd2631ea6f1685609fc66d50223cc696cb59ff2fee47ac`
	pk1 := MiraclToHerumiPK(miraclpk1)

	require.EqualValues(t, pk1, "68d37ed84842c91d9f82389489a1b1a7ab7a957816c635ee750769853aeddf1b490b3aa185a3f01f537cd1e9e56c43ab2617c8a3f9d2a1fd0dae23d26b2ca018")

	// Assert DeserializeHexStr works on the output of MiraclToHerumiPK
	var pk bls.PublicKey
	err := pk.DeserializeHexStr(pk1)
	require.NoError(t, err)
}

func TestMiraclToHerumiSig(t *testing.T) {
	miraclsig1 := `(0d4dbad6d2586d5e01b6b7fbad77e4adfa81212c52b4a0b885e19c58e0944764,110061aa16d5ba36eef0ad4503be346908d3513c0a2aedfd0d2923411b420eca)`
	sig1 := MiraclToHerumiSig(miraclsig1)

	require.EqualValues(t, sig1, "644794e0589ce185b8a0b4522c2181faade477adfbb7b6015e6d58d2d6ba4d0d")

	// Assert DeserializeHexStr works on the output of MiraclToHerumiSig
	var sig bls.Sign
	err := sig.DeserializeHexStr(sig1)
	require.NoError(t, err)

	// Test that passing in normal herumi sig just gets back the original.
	sig2 := MiraclToHerumiSig(sig1)
	if sig1 != sig2 {
		panic("Sigs should've been the same")
	}
}
