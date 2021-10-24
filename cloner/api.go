package cloner

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
)

type nexusApi struct {
	*http.Client
}

var (
	nxsErrRspNotFound = errors.New("Could not get the requested obj because of empty response from Nexus!")
	nxsErrRspTooMany  = errors.New("Coudld not get the requested obj because of too many returned objs from Nexus!")
	nxsErrRspUnknown  = errors.New("Could not get the requested obj because of Nexus abnormal response! Check logs for more information.")
	nxsErrRq404       = errors.New("Could not complete the request because of Nexus api respond 404 error!")
)

func NewNexusApi() *nexusApi {
	return &nexusApi{
		Client: &http.Client{
			Timeout: gCli.Duration("http-client-timeout"),
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: gCli.Bool("http-client-insecure"),
				},
			},
		},
	}
}

func (m *nexusApi) getNexusRequest(url string, rspJsonSchema interface{}) (e error) {
	var req *http.Request
	if req, e = http.NewRequest("GET", url, nil); e != nil {
		return
	}

	if len(gCli.String("srcRepoUsername")) > 0 && len(gCli.String("srcRepoPassword")) > 0 {
		req.SetBasicAuth(gCli.String("srcRepoUsername"), gCli.String("srcRepoPassword"))
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	gLog.Debug().Str("url", url).Msg("trying to make api request")

	var rsp *http.Response
	if rsp, e = m.Client.Do(req); e != nil {
		gLog.Warn().Msg("Could not get requested URL!")
		return
	}

	if rsp.StatusCode != http.StatusOK {
		gLog.Warn().Int("status", rsp.StatusCode).Msg("Abnormal API response! Check it immediately!")
		return nxsErrRq404
	}

	defer rsp.Body.Close()
	return m.parseNexusResponse(&rsp.Body, rspJsonSchema)
}

func (m *nexusApi) parseNexusResponse(rsp *io.ReadCloser, rspJsonSchema interface{}) (e error) {
	var data []byte
	if data, e = ioutil.ReadAll(*rsp); e != nil {
		return
	}

	return json.Unmarshal(data, &rspJsonSchema)
}
