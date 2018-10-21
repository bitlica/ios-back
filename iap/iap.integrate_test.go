// +build integrate

package iap

import (
	"context"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/stretchr/testify/require"
)

var secret = flag.String("secret", "", "ios app secret")

func init() {
	flag.Parse()
}
func TestTest(t *testing.T) {
	api := ReceiptService{
		IsSandbox: true,
		Secret:    *secret,
	}

	receipt, err := ioutil.ReadFile("testdata/1.bin")

	require.NoError(t, err)

	ctx := context.Background()
	subs, err := api.GetAutoRenewableIAPs(ctx, receipt)
	require.NoError(t, err)

	spew.Dump(subs)
}
