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
		//IsSandbox: true,
		Secret: *secret,
	}
	require.NotEmpty(t, *secret)

	receipt, err := ioutil.ReadFile("testdata/freshReceipt.txt")

	require.NoError(t, err)

	ctx := context.Background()

	//subs, err := api.GetAutoRenewableIAPs(ctx, receipt, ARActive|ARFree)
	//subs, err := api.GetAutoRenewableIAPs(ctx, receipt, ARActive|ARFree|ARExpired|ARCanceled)
	subs, err := api.GetAutoRenewableIAPs(ctx, receipt, ARExpired)
	require.NoError(t, err)

	spew.Dump(subs)
}
