package cloner

import (
	"errors"
	"net/url"
)

type nexus struct {
}

func NewNexus() *nexus { return &nexus{} }

func (m *nexus) getRepositoryStatus() {
}

func (m *nexus) getRepositoryAssets(repository string) (assets []*NexusAsset, e error) {

	var rrl *url.URL
	if rrl, e = url.Parse(gCli.String("srcRepoUrl") + "/service/rest/v1/assets"); e != nil {
		return
	}

	var rgs = &url.Values{}
	rgs.Set("repository", repository)
	rrl.RawQuery = rgs.Encode()

	var rsp *NexusAssetsCollection

	for {

		if e = gApi.getNexusRequest(rrl.String(), &rsp); e != nil {
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
