package cloner

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
)

type nexusApi struct {
	*http.Client

	username, password string
}

var (
	nxsErrRspNotFound = errors.New("Could not get the requested obj because of empty response from Nexus!")
	nxsErrRspUnknown  = errors.New("Could not get the requested obj because of Nexus abnormal response! Check logs for more information.")
	nxsErrRq404       = errors.New("Could not complete the request because of Nexus api respond 404 error!")
)

func newNexusApi(u, p string) *nexusApi {
	return &nexusApi{
		Client: &http.Client{
			Timeout: gCli.Duration("http-client-timeout"),
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: gCli.Bool("http-client-insecure"),
				},
			},
		},
		username: u,
		password: p,
	}
}

func (m *nexusApi) authorizeNexusRequest(r *http.Request) {
	if len(m.username) > 0 && len(m.password) > 0 {
		r.SetBasicAuth(m.username, m.password)
	}

	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json; charset=UTF-8")
}

func (m *nexusApi) getNexusRequest(url string, rspJsonSchema interface{}) (e error) {
	var req *http.Request
	if req, e = http.NewRequest("GET", url, nil); e != nil {
		return
	}

	m.authorizeNexusRequest(req)
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

func (m *nexusApi) getNexusFile(url string, file *os.File) (e error) {
	var req *http.Request
	if req, e = http.NewRequest("GET", url, nil); e != nil {
		return
	}

	m.authorizeNexusRequest(req)

	var rsp *http.Response
	if rsp, e = m.Client.Do(req); e != nil {
		return
	}

	if rsp.StatusCode != http.StatusOK {
		gLog.Warn().Int("status", rsp.StatusCode).Msg("Abnormal API response! Check it immediately!")
		return nxsErrRq404
	}

	defer rsp.Body.Close()
	if _, e = io.Copy(file, rsp.Body); e != nil {
		return
	}

	return
}

func (m *nexusApi) putNexusFile(url string, body *bytes.Buffer, contentType string) (e error) {
	var req *http.Request
	if req, e = http.NewRequest("POST", url, body); e != nil {
		return
	}

	m.authorizeNexusRequest(req)

	// rewrite authorize() content type with mime multipart content
	req.Header.Set("Content-Type", contentType)

	var rsp *http.Response
	if rsp, e = m.Client.Do(req); e != nil {
		return
	}

	if rsp.StatusCode != http.StatusOK {
		gLog.Warn().Int("status", rsp.StatusCode).Msg("Abnormal API response! Check it immediately!")

		if gIsDebug {
			if log, e := m.dumpNexusRequest(req, rsp); e != nil {
				gLog.Error().Err(e).Msg("WARNING! Could not dump http request and http response for debugging!")
			} else {
				fmt.Println(log)
			}
		}

		return nxsErrRq404
	}

	if gIsDebug {
		defer rsp.Body.Close()
		var rspBody []byte
		if rspBody, e = ioutil.ReadAll(rsp.Body); e != nil {
			return
		}

		gLog.Debug().Msg("Nexus API response:  " + string(rspBody))
	}

	return
}

func (m *nexusApi) dumpNexusRequest(r *http.Request, rsp *http.Response) (string, error) {
	var buf bytes.Buffer
	dump, e := httputil.DumpRequest(r, true)
	if e != nil {
		return "", e
	}
	buf.Write(dump)

	dump, e = httputil.DumpResponse(rsp, true)
	if e != nil {
		return "", e
	}

	buf.Write(dump)
	return buf.String(), nil
}
