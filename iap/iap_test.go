package iap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckStatusError(t *testing.T) {
	testcases := []struct {
		name   string
		rresp  ReceiptResponse
		expect *VerifyReceiptError
	}{
		{
			"no error status 0",
			ReceiptResponse{Status: 0},
			nil,
		},
		{
			"error in range: 21000",
			ReceiptResponse{Status: 21000},
			&VerifyReceiptError{fmt.Errorf(receiptErrors[21000]), 21000},
		},
		{
			"error in range: 21100-21199",
			ReceiptResponse{Status: 21100},
			&VerifyReceiptError{fmt.Errorf(defaultStatusError), 21100},
		},
		{
			"unknown status: 1",
			ReceiptResponse{Status: 1},
			&VerifyReceiptError{fmt.Errorf(unknownStatusError), 1},
		},
	}

	for _, tc := range testcases {
		vrerr := CheckStatusError(tc.rresp)
		require.Equal(t, vrerr, tc.expect)
	}
}
