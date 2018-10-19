package iap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	SandboxIAPURL = "https://sandbox.itunes.apple.com/verifyReceipt"
	ProdIAPURL    = "https://buy.itunes.apple.com/verifyReceipt"
)

type VerifyReceiptError struct {
	error
	Status string
}

var receiptErrors = map[string]string{
	"21000": "The App Store could not read the JSON object you provided.",
	"21002": "The data in the receipt-data property was malformed or missing.",
	"21003": "The receipt could not be authenticated.",
	"21004": "The shared secret you provided does not match the shared secret on file for your account.",
	"21005": "The receipt server is not currently available.",
	"21006": "This receipt is valid but the subscription has expired.", // for iOS 6 style transaction only, so we don't care
	"21007": "This receipt is from the test environment, but it was sent to the production environment for verification. Send it to the test environment instead.",
	"21008": "This receipt is from the production environment, but it was sent to the test environment for verification. Send it to the production environment instead.",
	"21010": "This receipt could not be authorized. Treat this the same as if a purchase was never made.",
}

// ReceiptService sends and parses response from apple service.
// You have to specify atleast the Secret.
// It is optimized for iOS 7 style app receipts.
type ReceiptService struct {
	IsSandbox              bool
	Secret                 string
	ExcludeOldTransactions bool
	MaxRetry               int // retry if get status code 21100-21199
}

func (rs ReceiptService) GetAutoRenewableIAPs(ctx context.Context, receipt []byte) ([]AutoRenewable, error) {
	rreq := ReceiptRequest{
		ReceiptData:            string(receipt),
		Password:               rs.Secret,
		ExcludeOldTransactions: rs.ExcludeOldTransactions,
	}

	rresp, err := rs.VerifyReceipt(ctx, rreq)
	if err != nil {
		return nil, err
	}

	iaps, err := rresp.ParseLatestReceiptInfo()
	if err != nil {
		return nil, err
	}

	result := ExtractAutoRenewable(iaps)
	return result, nil
}

// recomended approach to check snadbox request
// see https://developer.apple.com/library/archive/documentation/NetworkingInternet/Conceptual/StoreKitGuide/Chapters/AppReview.html
// do not need concurrent requests, since for the test env it's ok to have some lag.
func (rs ReceiptService) VerifyReceipt(ctx context.Context, rreq ReceiptRequest) (ReceiptResponse, error) {
	if rs.IsSandbox {
		return VerifyReceipt(ctx, SandboxIAPURL, rs.MaxRetry, rreq)
	}

	rresp, err := VerifyReceipt(ctx, ProdIAPURL, rs.MaxRetry, rreq)
	if rresp.Status == "21007" {
		return VerifyReceipt(ctx, SandboxIAPURL, rs.MaxRetry, rreq)
	}
	return rresp, err
}

func VerifyReceipt(ctx context.Context, url string, maxretry int, rreq ReceiptRequest) (ReceiptResponse, error) {
	rresp := ReceiptResponse{}

	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(rreq); err != nil {
		return rresp, err
	}

	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		return rresp, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return rresp, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&rresp); err != nil {
		return rresp, err
	}

	// check receipt status and retry if needed
	if rresp.Status != "0" && rresp.IsRetryable && maxretry > 0 {
		maxretry--
		return VerifyReceipt(ctx, url, maxretry, rreq)
	}

	return rresp, nil
}

func CheckStatusError(rresp ReceiptResponse) error {
	if rresp.Status == "0" {
		return nil
	}

	errmasg, ok := receiptErrors[rresp.Status]
	if !ok {
		errmasg = "Internal data access error. status=" + rresp.Status
	}

	return VerifyReceiptError{
		fmt.Errorf(errmasg),
		rresp.Status,
	}
}

type ReceiptRequest struct {
	ReceiptData            string `json:"receipt-data"`
	Password               string `json:"password,omitempty"`
	ExcludeOldTransactions bool   `json:"exclude-old-transactions,omitempty"`
}

type ReceiptResponse struct {
	Status             string          `json:"status"`
	IsRetryable        bool            `json:"is-retryable"` // if status 21100-21199
	ReceiptContent     json.RawMessage `json:"receipt"`
	LatestReceipt      json.RawMessage `json:"latest_receipt"`       // the latest base-64 encoded app receipt.
	LatestReceiptInfo  json.RawMessage `json:"latest_receipt_info"`  // returned for receipts containing auto-renewable subscriptions. array containing all in-app purchase transactions.  excludes finished consumables.
	PendingRenewalInfo json.RawMessage `json:"pending_renewal_info"` // pending renewal information for each auto-renewable subscription identified by the Product Identifier. Refers to a renewal scheduled in the future or failed in the past.
	// latest_expired_receipt_info - Only returned for iOS 6 style
}

func parse(rresp ReceiptResponse, data json.RawMessage, obj interface{}) error {
	if err := CheckStatusError(rresp); err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

func (rresp ReceiptResponse) ParseReceiptContent() (Receipt, error) {
	var iaps Receipt
	err := parse(rresp, rresp.ReceiptContent, &iaps)
	return iaps, err
}
func (rresp ReceiptResponse) ParseLatestReceipt() (Receipt, error) {
	var iaps Receipt
	err := parse(rresp, rresp.LatestReceipt, &iaps)
	return iaps, err
}
func (rresp ReceiptResponse) ParseLatestReceiptInfo() ([]InApp, error) {
	var iaps []InApp
	err := parse(rresp, rresp.LatestReceiptInfo, &iaps)
	return iaps, err
}

func (rresp ReceiptResponse) ParsePendingRenewalInfo() ([]InApp, error) {
	var iaps []InApp
	err := parse(rresp, rresp.PendingRenewalInfo, &iaps)
	return iaps, err
}

// The structure of returned receipt according to https://developer.apple.com/library/archive/releasenotes/General/ValidateAppStoreReceipt/Chapters/ReceiptFields.html
// The structure is in sync with revision 2017-12-11 , see https://developer.apple.com/library/archive/releasenotes/General/ValidateAppStoreReceipt/Chapters/RevisionHistory.html
type Receipt struct {
	BundleID                   string `json:"bundle_id"`                    //  CFBundleIdentifier in the Info.plist file. Use to validate the receipt was indeed generated for the app.
	ApplicationVersion         string `json:"application_version"`          // CFBundleVersion (in iOS) or CFBundleShortVersionString (in macOS) in the Info.plist.
	OriginalApplicationVersion string `json:"original_application_version"` // In sandbox = “1.0”, CFBundleVersion (in iOS) or CFBundleShortVersionString (in macOS) in the Info.plist file when the purchase was originally made.

	// empty array is valid	// IAP receipt for a consumable is temporal.
	InApp []InApp `json:"in_app"` // in-app purchase transactions: 1) from input base-64 receipt-data. 2) or in latest_receipt_info response (preferable for auto-renewable)

	ReceiptCreationDate   json.RawMessage `json:"receipt_creation_date"` // when the app receipt was created. used to validate receipt’s signature. interpreted as an RFC 3339 date
	ReceiptExpirationDate json.RawMessage `json:"expiration_date"`       // check receipt expiration: compare this date to the current date. key is present for apps purchased through the Volume Purchase Program
}

// All fields starting with Subscription* in fact applicable only to auto-renewable purshases
// All fields names corresponds to json names except for Subscriptions*
type InApp struct {
	AppItemID                 string          `json:"app_item_id"`                 // present only in prod for IOS apps. identify the application that created the transaction. See also Bundle Identifier.
	VersionExternalIdentifier string          `json:"version_external_identifier"` // present only in prod. An arbitrary number. Use this value to identify the version of the app that the customer bought.
	WebOrderLineItemID        string          `json:"web_order_line_item_id"`      // This value is a unique ID that identifies purchase events across devices, including subscription renewal purchase events.
	Quantity                  int             `json:"quantity,string"`             // The number of items purchased.
	ProductID                 string          `json:"product_id"`
	TransactionID             string          `json:"transaction_id"`          // In an auto-renewable subscription receipt, a new value for the transaction identifier is generated every time the subscription automatically renews or is restored on a new device.
	OriginalTransactionID     string          `json:"original_transaction_id"` // This value is the same for all receipts that have been generated for a specific subscription.
	PurchaseDate              json.RawMessage `json:"purchase_date"`           //  RFC 3339 date. In an auto-renewable subscription receipt, the purchase date is the start date of the next period, which is identical to the end date of the current period.
	OriginalPurchaseDate      json.RawMessage `json:"original_purchase_date"`

	// For a transaction that was canceled by Apple customer support, the time and date of the cancellation.
	// For an auto-renewable subscription plan that was upgraded, the time and date of the upgrade transaction.
	// Note: A canceled in-app purchase remains in the receipt indefinitely. Only applicable if the refund was for a non-consumable product, an auto-renewable subscription, a non-renewing subscription, or for a free subscription.
	CancellationDate json.RawMessage `json:"cancellation_date"` // RFC 3339 . Treat a canceled receipt the same as if no purchase had ever been made.

	// “1” - Customer canceled their transaction due to an actual or perceived issue within your app.
	// “0” - Transaction was canceled for another reason, for example, if the customer made the purchase accidentally.
	CancellationReason int `json:"cancellation_reason,string"` // Use this value along with the cancellation date to identify possible issues in your app that may lead customers to contact Apple customer support.

	//only for auto-renewable. identify the date when subscription will renew or expire,  past date means expired.
	SubscriptionExpirationDate json.RawMessage `json:"expires_date` // unix timestamp. RFC 3339 date. The expiration date for the subscription,

	//“1” - Customer canceled their subscription.
	//“2” - Billing error; for example customer’s payment information was no longer valid.
	//“3” - Customer did not agree to a recent price increase.
	//“4” - Product was not available for purchase at the time of renewal.
	//“5” - Unknown error.
	SubscriptionExpirationIntent int `json:"expiration_intent,string"` // only present for an expired subscription, the reason of expiration.

	// “1” - Customer has agreed to the price increase. Subscription will renew at the higher price.
	// “0” - Customer has not taken action regarding the increased price. Subscription expires if the customer takes no action before the renewal date.
	SubscriptionPriceConsentStatus int `json:"price_consent_status,string"` // only for auto-renewable if the subscription price was increased without keeping the existing price for active subscribers.

	// “1” - App Store is still attempting to renew the subscription.
	// “0” - App Store has stopped attempting to renew the subscription.
	SubscriptionRetryFlag int `json:"is_in_billing_retry_period,string"` //only present for an expired subscription, whether or not Apple is still attempting to automatically renew the subscription.

	// Note: If a previous subscription period in the receipt has the value “true” for either the is_trial_period or the is_in_intro_offer_period key,
	// the user is not eligible for a free trial or introductory price within that subscription group.
	SubscriptionTrialPeriod bool `json:"is_trial_period,string"` // only for auto-renewable subscription receipts. "true" if free trial period, or "false" if not.

	// Note: If a previous subscription period in the receipt has the value “true” for either the is_trial_period or the is_in_intro_offer_period key,
	// the user is not eligible for a free trial or introductory price within that subscription group.
	SubscriptionIntroductoryPricePeriod bool `json:"is_in_intro_offer_period,string"` // only for auto-renewable subscription receipts. "true" if an introductory price period, or "false" if not.

	// “1” - Subscription will renew at the end of the current subscription period.
	// “0” - Customer has turned off automatic renewal for their subscription.
	SubscriptionAutoRenewStatus     int    `json:"auto_renew_status,string"` // only for auto-renewable.  The current renewal status for the auto-renewable subscription.
	SubscriptionAutoRenewPreference string `json:"auto_renew_product_id"`    // only for auto-renewable. You can use this value to present an alternative service level to the customer before the current subscription period ends.
}

type AutoRenewable struct {
	InApp
	State ARState
}

type ARState byte

const (
	AROk ARState = iota
	ARFree
	ARExpired
	ARCanceled
)

func ExtractAutoRenewable(iaps []InApp) []AutoRenewable {
	subs := make([]AutoRenewable, 0, len(iaps))
	for _, p := range iaps {
		state := getState(p)
		sub := AutoRenewable{p, state}
		subs = append(subs, sub)
	}
	return subs
}

func getState(p InApp) ARState {
	switch {
	case len(p.CancellationDate) > 0:
		return ARCanceled
	case p.SubscriptionExpirationIntent > 0:
		return ARExpired
	case p.SubscriptionTrialPeriod || p.SubscriptionIntroductoryPricePeriod:
		return ARFree
	default:
		return AROk
	}
}
