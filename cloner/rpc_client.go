package cloner

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

type rpcClient struct {
	*http.Client
}

func newRpcClient() *rpcClient {
	return &rpcClient{
		Client: &http.Client{
			Timeout: gCli.Duration("http-client-timeout"),
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: gCli.Bool("http-client-insecure"),
				},
				DisableCompression: false,
			},
		},
	}
}

func (m *rpcClient) parseResponsePayload(rsp *io.ReadCloser, rspPayload interface{}) (e error) {
	var data []byte
	if data, e = ioutil.ReadAll(*rsp); e != nil {
		return
	}

	return json.Unmarshal(data, &rspPayload)
}

func (m *rpcClient) postRpcRequest(url string, body *bytes.Buffer, rspPayload interface{}) (e error) {
	var req *http.Request
	if req, e = http.NewRequest("POST", url, body); e != nil {
		return
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	var rsp *http.Response
	if rsp, e = m.Client.Do(req); e != nil {
		return
	}

	if rsp.StatusCode != http.StatusOK {
		gLog.Warn().Int("status", rsp.StatusCode).Msg("Abnormal RPC response! Check it immediately!")
	}

	defer rsp.Body.Close()
	return m.parseResponsePayload(&rsp.Body, rspPayload)
}
