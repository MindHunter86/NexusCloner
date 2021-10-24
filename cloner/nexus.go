package cloner

import (
	"errors"
	"net/url"
)

type nexus struct {
	url              string
	username         string
	password         string
	repositoryName   string
	assetsCollection []*NexusAsset

	api *nexusApi
}

func newNexus(ur, us, p, rn string) *nexus {
	api := newNexusApi(us, p)
	return &nexus{
		url:            ur,
		username:       us,
		password:       p,
		repositoryName: rn,
		api:            api,
	}
}

func (m *nexus) getRepositoryStatus() (e error) {
	var rrl *url.URL
	if rrl, e = url.Parse(m.url + "/status"); e != nil {
		gLog.Warn().Str("url", m.url).Err(e).Msg("Abnormal status from repository")
		return
	}

	if e = m.api.getNexusRequest(rrl.String(), nil); e != nil {
		if e == nxsErrRq404 {
			gLog.Error().Err(e).Msg("Given Nexus server is avaliable but has abnormal response code. Check it manually.")
			return nxsErrRspUnknown
		}
		gLog.Error().Err(e).Msg("There is some troubles with repository availability")
		return
	}

	return
}

func (m *nexus) getRepositoryAssets(repository string) (assets []*NexusAsset, e error) {
	if e := m.getRepositoryStatus(); e != nil {
		return nil, e
	}

	var rrl *url.URL
	if rrl, e = url.Parse(gCli.String("srcRepoUrl") + "/service/rest/v1/assets"); e != nil {
		return
	}

	var rgs = &url.Values{}
	rgs.Set("repository", repository)
	rrl.RawQuery = rgs.Encode()

	var rsp *NexusAssetsCollection

	for {
		if e = m.api.getNexusRequest(rrl.String(), &rsp); e != nil {
			if e == nxsErrRq404 {
				return nil, nxsErrRspNotFound
			}
			return
		}

		if rsp.Items == nil {
			gLog.Error().Msg("Internal error, assets are empty after api parsing")
			return nil, errors.New("Internal error, assets are empty after api parsing")
		}

		assets = append(assets, rsp.Items...)
		gLog.Info().Int("buffer", len(assets)).Int("assets", len(rsp.Items)).Msg("successfully parsed assets")

		if len(rsp.ContinuationToken) == 0 {
			break
		}

		rgs.Set("continuationToken", rsp.ContinuationToken)
		rrl.RawQuery = rgs.Encode()
		rsp.ContinuationToken = ""
	}

	gLog.Info().Int("count", len(assets)).Msg("Successfully parsed src repository assets")
	return
}
