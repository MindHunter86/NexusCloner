package cloner

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
)

type rpcClient struct {
	*http.Client

	mu         sync.RWMutex
	requestTid int
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
		requestTid: 0,
	}
}

func (m *rpcClient) newRpcRequest(method string, data *map[string]string) *rpcRequest {
	m.mu.Lock() // avoid race cond
	tid := m.requestTid + 1
	m.requestTid = tid
	m.mu.Unlock()

	return &rpcRequest{
		Action: "coreui_Browse",
		Tid:    tid,
		Type:   "rpc",
		Method: method,
		Data:   *data,
	}
}

func (m *rpcClient) newRpcJsonRequest(method string, data interface{}) ([]byte, error) {
	m.mu.Lock() // avoid race cond
	tid := m.requestTid + 1
	m.requestTid = tid
	m.mu.Unlock()

	return json.Marshal(&rpcRequest{
		Action: "coreui_Browse",
		Type:   "rpc",
		Tid:    tid,
		Method: method,
		Data:   data,
	})
}

func (m *rpcClient) parseResponsePayload(rsp *io.ReadCloser, rspPayload interface{}) (e error) {
	var data []byte
	if data, e = ioutil.ReadAll(*rsp); e != nil {
		return
	}

	return json.Unmarshal(data, &rspPayload)
}

func (m *rpcClient) postRpcRequest(url string, reqPayload []byte, rspPayload interface{}) (e error) {
	var req *http.Request
	if req, e = http.NewRequest("POST", url, bytes.NewBuffer(reqPayload)); e != nil {
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
