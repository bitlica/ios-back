package iap

// structure according to https://developer.apple.com/documentation/storekit/in-app_purchase/enabling_status_update_notifications
// DSIS - didn't see in sandbox - mark for fields.
// some of the field is undocumented
type Notification struct {
	//	NotificationType:
	// INITIAL_BUY - Occurs at the initial purchase of the subscription. Store the latest_receipt on your server as a token to verify the user’s subscription status at any time, by validating it with the App Store.
	// CANCEL - Indicates that the subscription was canceled either by Apple customer support or by the App Store when the user upgraded their subscription. The cancellation_date key contains the date and time when the subscription was canceled or upgraded.
	// RENEWAL - Indicates successful automatic renewal of an expired subscription that failed to renew in the past. Check subscription_expiraton_date to determine the next renewal date and time.
	// INTERACTIVE_RENEWAL - Indicates the customer renewed a subscription interactively, either by using your app’s interface, or on the App Store in account settings. Make service available immediately.
	// DID_CHANGE_RENEWAL_PREF - Indicates the customer made a change in their subscription plan that takes effect at the next renewal. The currently active plan is not affected.
	// DID_CHANGE_RENEWAL_STATUS - Indicates a change in the subscription renewal status. Check the timestamp for the data and time of the latest status update, and the auto_renew_status for the current renewal status.
	// unsubscribe : DID_CHANGE_RENEWAL_STATUS + AutoRenewStatus=false
	NotificationType string `json:"notification_type"`

	Environment           string `json:"environment"`             // PROD | Sandbox
	Password              string `json:"password"`                // aka shared secret
	OriginalTransactionID string `json:"original_transaction_id"` // DSIS. the same as in latest_receipt_info. Use it to relate multiple iOS 6-style transaction receipts for an individual customer’s subscription.
	WebOrderLineItemID    string `json:"web_order_line_item_id"`  // DSIS. The primary key for identifying a subscription purchase.

	// Posted only if the notification_type is CANCEL.
	CancellationDate Time `json:"cancellation_date_ms"` // DSIS. when cancelled by Apple customer support.

	// Posted if the notification_type is RENEWAL or INTERACTIVE_RENEWAL, and only if the renewal is successful.
	// Posted also if the notification_type is INITIAL_BUY.
	// Not posted for notification_type CANCEL.
	LatestReceipt     string  `json:"latest_receipt"`      // most recent base-64. Posted only if the notification_type is RENEWAL or INTERACTIVE_RENEWAL, and only if the renewal is successful.
	LatestReceiptInfo InAppV6 `json:"latest_receipt_info"` // most recent json.  Posted only if renewal is successful. Not posted for notification_type CANCEL.

	// Posted only if the notification_type is RENEWAL or CANCEL or if renewal failed and subscription expired.
	LatestExpiredReceipt     string  `json:"latest_expired_receipt"`      // DSIS. most recent renewal base-64. Posted only if the subscription expired.
	LatestExpiredReceiptInfo InAppV6 `json:"latest_expired_receipt_info"` // DSIS. most recent renewal json. Posted only if the notification_type is RENEWAL or CANCEL or if renewal failed and subscription expired.

	// Check it when DID_CHANGE_RENEWAL_STATUS happend
	AutoRenewStatus           string `json:"auto_renew_status"`                // false or true. the same as for receipt.
	AutoRenewStatusChangeDate Time   `json:"auto_renew_status_change_date_ms"` // UNDOCUMENTED

	// Check it when DID_CHANGE_RENEWAL_PREF happend
	AutoRenewProductID string `json:"auto_renew_product_id"` // This is the same as the Subscription Auto Renew Preference in the receipt. See also Receipt Fields.
	AutoRenewAdamID    string `json:"auto_renew_adam_id"`    // DSIS. The current renewal preference for the auto-renewable subscription. This is the Apple ID of the product.

	// present if RENEWAL or INTERACTIVE_RENEWAL
	ExpirationIntent string `json:"expiration_intent"` // DSIS. reason of expiration. This is the same as the Subscription Expiration Intent in the receipt. Posted only if notification_type is RENEWAL or INTERACTIVE_RENEWAL. See also Receipt Fields.
}

func (n Notification) GetSupscription() InAppV6 {
	if n.LatestReceipt == "" {
		// assume type Cancel
		return n.LatestExpiredReceiptInfo
	}

	return n.LatestReceiptInfo
}

type InAppV6 struct {
	// inside notification it seem doesn't include CancellationDate and CancellationReason
	// though not clear for other cases
	InApp

	ItemID         string `json:"app_item_id"`  // is it instead app_item_id ?
	ExpirationDate Time   `json:"expires_date"` // instead of expires_date_ms

	// Additional iOS 6 style fields
	UniqueVendorIdentifier string `json:"unique_vendor_identifier"` // e.g. "FC40A4BA-F5B2-4FC0-95E5-1179A9DE7003"
	UniqueIdentifier       string `json:"unique_identifier"`        // e.g. "1ba0ac3365f1e1b634d7ba0d35bda8fcbaf4c85f"
	BVRS                   string `json:"bvrs"`                     // the same as Receipt's original_application_version e.g. "46"
	BID                    string `json:"bid"`                      // the same as Receipt's bundle_id
}

func (iap InAppV6) ToV7() InApp {
	iap.AppItemID = iap.ItemID
	iap.ExpirationDate = iap.SubscriptionExpirationDate
	return iap.InApp
}
