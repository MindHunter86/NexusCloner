package cloner

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
)

var (
	errNxsDwnlErrs = errors.New("Download process has not successfully finished. Check logs and restart program. Also u can use --skip-download-errors flag.")
	errInvGivArg   = errors.New("There is some problems with parsing you repository endpoint. Make sure, that you give correct data.")
)

type nexus struct {
	endpoint         *url.URL
	repository, path string
	assetsCollection []*NexusAsset

	api      *nexusApi
	tempPath string
}

func newNexus() *nexus {
	return &nexus{
		api: newNexusApi(),
	}
}

// schema: https://username:password@nexus.example.com:3131/test-repository/com/example/components/v1.2.....
func (m *nexus) initiate(arg string) (*nexus, error) {
	var e error
	m.endpoint, e = url.Parse(arg)
	if e != nil {
		return nil, e
	}

	// get repository name and path for futher removing from url
	buf := strings.Split(m.endpoint.EscapedPath(), "/")
	m.repository, m.path = buf[1], strings.Join(buf[2:], "/")

	gLog.Debug().Str("url", m.endpoint.Redacted()).Msg("parsed url")
	gLog.Debug().Str("encpath", m.endpoint.EscapedPath()).Msg("truncate url path")

	m.endpoint.RawPath = ""
	m.endpoint.RawQuery = ""

	gLog.Debug().Str("repo", m.repository).Str("path", m.path).Msg("testing given repo/path")

	if len(m.repository) == 0 {
		return nil, errInvGivArg
	}

	if len(m.path) == 0 {
		m.path = gCli.String("path-filter")
	}

	// check for user+password
	uPass, _ := m.endpoint.User.Password()
	if len(m.endpoint.User.Username()) == 0 || len(uPass) == 0 {
		gLog.Warn().Msg("There is empty credentials for the repository. I'll hope, it's okay.")
	}

	return m, nil
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
	if rrl, e = m.endpoint.Parse("/service/rest/v1/status"); e != nil {
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
	if rrl, e = m.endpoint.Parse("/service/rest/v1/search/assets"); e != nil {
		return
	}

	var rgs = &url.Values{}
	rgs.Set("repository", m.repository)
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

		var r *regexp.Regexp
		if r, e = regexp.Compile(m.path); e != nil {
			return
		}

		for _, asset := range rsp.Items {
			if r.MatchString(asset.Path) {
				if matched, _ := regexp.MatchString("((maven-metadata\\.xml)|\\.(pom|md5|sha1|sha256|sha512))$", asset.Path); matched {
					gLog.Debug().Msgf("The asset %s will be skipped!", asset.Path)
					continue
				}

				gLog.Debug().Str("path", asset.Path).Msg("Asset path matched!")
				assets = append(assets, asset)
			} else {
				gLog.Debug().Str("path", asset.Path).Msg("Asset path NOT matched!")
			}
		}

		// assets = append(assets, rsp.Items...)
		gLog.Info().Int("total_matched_by_path", len(assets)).Int("step_parsed", len(rsp.Items)).Msg("Successfully parsed page")

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
	if len(gCli.String("process-continue-directory")) != 0 {
		m.tempPath = gCli.String("process-continue-directory")
		return
	}

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

		gLog.Debug().Msg("asset - " + asset.getHumanReadbleName())

		// TODO refactor!
		if asset.Maven2 != nil {
			if len(asset.Maven2.Extension) == 0 {
				gLog.Warn().Msgf("The file %s has strange metadata. Check it please and try again later.", asset.getHumanReadbleName())
				continue
			}
		} else {
			gLog.Warn().Msgf("The file %s has strange metadata. Check it please and try again later.", asset.getHumanReadbleName())
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
		if rrl, e = m.endpoint.Parse("/service/rest/v1/components"); e != nil {
			return
		}

		var rgs = &url.Values{}
		rgs.Set("repository", m.repository)
		rrl.RawQuery = rgs.Encode()

		if e = m.api.putNexusFile(rrl.String(), body, contentType); e != nil {
			isErrored = true
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
	var mw = multipart.NewWriter(buf) // TODO BUG with pointers?
	defer mw.Close()

	for k, v := range meta {
		var fw io.Writer
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
	return
}

func (m *nexus) setTemporaryDirectory(tdir string) {
	m.tempPath = tdir
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
