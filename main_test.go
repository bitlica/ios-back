package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"github.com/Loofort/ios-back/iap"
	"github.com/stretchr/testify/require"
)

func TestTest(t *testing.T) {
	// we mock apple server, so we don't need to provide the secret
	rs := iap.ReceiptService{
		Client: &http.Client{Transport: &appleMock{}},
	}

	servemux := serveMux(rs)
	ts := httptest.NewServer(servemux)
	defer ts.Close()

	// get token
	req := tokenRequest(t, ts.URL+"/token")
	resp := apicall(t, req)
	token := resp["access_token"].(string)

	// use token
	req, err := http.NewRequest("GET", ts.URL+"/user", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", token))

	resp = apicall(t, req)
	if resp["uid"].(string) == "" {
		t.Fatalf("unexpected reponse: %v", resp)
	}
}

func tokenRequest(t *testing.T, url string) *http.Request {
	content, err := ioutil.ReadFile("stub_receipt.json")
	require.NoError(t, err)
	receipt := base64.StdEncoding.EncodeToString(content)

	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)

	err = w.WriteField("bundle_id", bundleID)
	require.NoError(t, err)

	err = w.WriteField("identifier_for_vendor", "some-fictional-device-id")
	require.NoError(t, err)

	fw, err := w.CreateFormFile("receipt", "doesnt_matter_name.bin")
	require.NoError(t, err)
	fw.Write([]byte(receipt))

	err = w.Close()
	require.NoError(t, err)

	req, err := http.NewRequest("POST", url, body)
	require.NoError(t, err)

	req.Header.Add("Content-Type", w.FormDataContentType())
	return req
}

func apicall(t *testing.T, req *http.Request) map[string]interface{} {
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respdump, err := httputil.DumpResponse(resp, true)
		require.NoError(t, err)
		t.Fatalf("unexpected api response\nresp:%s", respdump)
	}

	respmap := map[string]interface{}{}
	err = json.NewDecoder(resp.Body).Decode(&respmap)
	require.NoError(t, err)

	return respmap
}

// appleMock is http transport that mock apple's requests.
// it decodes base64 request body and return it as a response
type appleMock struct{}

// Implement http.RoundTripper
func (t *appleMock) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create mocked http.Response

	defer req.Body.Close()
	// preserve request body for debugging purposes
	//buf := new(bytes.Buffer)
	//tee := io.TeeReader(req.Body, buf)
	//req.Body = ioutil.NopCloser(tee)
	//defer func() { req.Body = ioutil.NopCloser(buf) }()

	rreq := iap.ReceiptRequest{}
	if err := json.NewDecoder(req.Body).Decode(&rreq); err != nil {
		return nil, err
	}

	b, err := base64.StdEncoding.DecodeString(rreq.ReceiptData)
	if err != nil {
		return nil, err
	}

	response := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	response.Header.Set("Content-Type", "application/json")
	response.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	return response, nil
}
