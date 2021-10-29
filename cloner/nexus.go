package cloner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/url"
	"os"
	"runtime"
	"strings"
)

var (
	errNxsDwnlErrs = errors.New("Download process has not successfully finished. Check logs and restart program. Also u can use --skip-download-errors flag.")
)

type nexus struct {
	url              string
	username         string
	password         string
	repositoryName   string
	assetsCollection []*NexusAsset

	api      *nexusApi
	tempPath string
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

func (m *nexus) destruct() {
	if len(m.tempPath) != 0 && !gCli.Bool("temp-path-save") {
		if e := os.RemoveAll(m.tempPath); e != nil {
			gLog.Warn().Err(e).Msg("There is some errors in Destruct() function. Looks bad.")
		}
	}
}

func (m *nexus) getRepositoryStatus() (e error) {
	var rrl *url.URL
	if rrl, e = url.Parse(m.url + "/service/rest/v1/status"); e != nil {
		gLog.Warn().Str("url", m.url).Err(e).Msg("Abnormal status from repository")
		return
	}

	if e = m.api.getNexusRequest(rrl.String(), struct{}{}); e != nil {
		if e == nxsErrRq404 {
			gLog.Error().Err(e).Msg("Given Nexus server is avaliable but has abnormal response code. Check it manually.")
			return nxsErrRspUnknown
		}
		gLog.Error().Err(e).Msg("There is some troubles with repository availability")
		return
	}

	return
}

func (m *nexus) getRepositoryAssets() (assets []*NexusAsset, e error) {
	// !!!
	// !!!
	// !!!
	// if e = m.getRepositoryStatus(); e != nil {
	// 	return
	// }

	var rrl *url.URL
	if rrl, e = url.Parse(m.url + "/service/rest/v1/assets"); e != nil {
		return
	}

	var rgs = &url.Values{}
	rgs.Set("repository", m.repositoryName)
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
		rsp = nil
	}

	gLog.Info().Int("count", len(assets)).Msg("Successfully parsed repository assets")
	return
}

func (m *nexus) createTemporaryDirectory() (e error) {
	pathPrefix := gCli.String("temp-path-prefix")
	if runtime.GOOS == "linux" && len(pathPrefix) == 0 {
		pathPrefix = "/var/tmp"
	}

	gLog.Debug().Msgf("creating temporary path with %s prefix", pathPrefix)
	if m.tempPath, e = ioutil.TempDir(pathPrefix, "*"); e != nil {
		return
	}

	return
}

func (m *nexus) getTemporaryDirectory() string {
	return m.tempPath
}

// TODO
// show download progress
// https://golangcode.com/download-a-file-with-progress/ - example
func (m *nexus) downloadMissingAssets(assets []*NexusAsset) (e error) {
	var dwnListCount = len(assets)
	var downloaded, errors int

	for _, asset := range assets {
		var file *os.File
		if file, e = asset.getTemporaryFile(m.tempPath); e != nil {
			gLog.Error().Err(e).Msgf("There is error while allocating temporary asset file. Asset %s will be skipped.", asset.ID)
			errors++
			continue
		}
		defer file.Close()

		var rrl *url.URL
		if rrl, e = url.Parse(asset.DownloadURL); e != nil {
			return
		}

		if e = m.api.getNexusFile(rrl.String(), file); e != nil {
			gLog.Error().Err(e).Msgf("There is error while downloading asset. Asset %s will be skipped.", asset.ID)
			errors++
			continue
		}

		downloaded++
		gLog.Info().Msgf("%s file has been downloaded successfully. Remaining %d files.", asset.getHumanReadbleName(), dwnListCount-downloaded)
	}

	if errors > 0 {
		gLog.Warn().Msgf("There was %d troubles with file downloading. Check logs and try again later.", errors) // TODO retranslate
		if !gCli.Bool("skip-download-errors") {
			return errNxsDwnlErrs
		}
	}

	gLog.Info().Msgf("Missing assets successfully . Downloaded %d files.", downloaded)
	return nil
}

func (m *nexus) uploadMissingAssets(assets []*NexusAsset) (e error) {
	var isErrored bool
	var assetsCount = len(assets)

	for _, asset := range assets {
		var file *os.File
		if file, e = asset.isFileExists(m.tempPath); e != nil {
			isErrored = true
			gLog.Error().Err(e).Str("filename", asset.getHumanReadbleName()).
				Msg("Could not find the asset's file. Asset will be skipped!")
			continue
		}

		var fileApiMeta = make(map[string]io.Reader)
		fileApiMeta["asset0"] = file
		fileApiMeta["asset0.extension"] = strings.NewReader(asset.Maven2.Extension)
		fileApiMeta["groupId"] = strings.NewReader(asset.Maven2.GroupID)
		fileApiMeta["artifactId"] = strings.NewReader(asset.Maven2.ArtifactID)
		fileApiMeta["version"] = strings.NewReader(asset.Maven2.Version)

		var body *bytes.Buffer
		var contentType string
		if body, contentType, e = m.getNexusFileMeta(fileApiMeta); e != nil {
			isErrored = true
			gLog.Error().Err(e).Str("filename", asset.getHumanReadbleName()).
				Msg("Could not get meta data for the asset's file.")
			continue
		}

		var rrl *url.URL
		if rrl, e = url.Parse(m.url + "/service/rest/v1/components"); e != nil {
			return
		}

		var rgs = &url.Values{}
		rgs.Set("repository", m.repositoryName)
		rrl.RawQuery = rgs.Encode()

		if e = m.api.putNexusFile(rrl.String(), body, contentType); e != nil {
			isErrored = true
			fmt.Println("dump")
			fmt.Println(body.String())
			continue
		}

		assetsCount--
		gLog.Info().Msgf("The asset %s has been uploaded successfully. Remaining %d files", asset.getHumanReadbleName(), assetsCount)
	}

	if isErrored {
		gLog.Warn().Msg("There was some errors in the upload proccess. Check logs and try again.")
	}

	return
}

func (m *nexus) getNexusFileMeta(meta map[string]io.Reader) (buf *bytes.Buffer, contentType string, e error) {
	buf = bytes.NewBuffer([]byte(""))
	var buf2 bytes.Buffer
	var mw = multipart.NewWriter(&buf2) // TODO BUG with pointers?
	defer mw.Close()

	for k, v := range meta {
		var fw io.Writer
		fmt.Println(k)
		fmt.Println(v)

		// !!
		// TODO this shit from google. I dont know about DEFER in IF!
		if x, ok := v.(io.Closer); ok {
			defer x.Close()
		}

		if x, ok := v.(*os.File); ok {
			if fw, e = mw.CreateFormFile(k, x.Name()); e != nil {
				return
			}
		} else {
			if fw, e = mw.CreateFormField(k); e != nil {
				return
			}
		}

		if _, e = io.Copy(fw, v); e != nil {
			return
		}
	}

	contentType = mw.FormDataContentType()
	return &buf2, contentType, nil
}

/*	Google + stackoverflow shit :
// Prepare a form that you will submit to that URL.
var b bytes.Buffer
w := multipart.NewWriter(&b)
for key, r := range values {
    var fw io.Writer
    if x, ok := r.(io.Closer); ok {
        defer x.Close()
    }
    // Add an image file
    if x, ok := r.(*os.File); ok {
        if fw, err = w.CreateFormFile(key, x.Name()); err != nil {
            return
        }
    } else {
        // Add other fields
        if fw, err = w.CreateFormField(key); err != nil {
            return
        }
    }
    if _, err = io.Copy(fw, r); err != nil {
        return err
    }

}
// Don't forget to close the multipart writer.
// If you don't close it, your request will be missing the terminating boundary.
w.Close()
*/
