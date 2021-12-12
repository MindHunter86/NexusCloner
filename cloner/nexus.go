package cloner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var (
	errNxsDwnlErrs = errors.New("Download process has not successfully finished. Check logs and restart program. Also u can use --skip-download-errors flag.")
	errInvGivArg   = errors.New("There is some problems with parsing you repository endpoint. Make sure, that you give correct data.")
)

type nexus struct {
	endpoint         *url.URL
	repository, path string

	api      *nexusApi
	tempPath string

	mu                       sync.RWMutex
	assetsCollection         []NexusAsset2
	dwnedAssets, upledAssets int
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

// !!
// TODO WARN RESULT IS EMPTY !!
func (m *nexus) getRepositoryAssetsRPC(path string) (e error) {
	var rpcPayload = map[string]string{
		"node":           path,
		"repositoryName": m.repository,
	}

	var rpcResponse *rpcRsp
	if rpcResponse, e = m.getRepositoryDataRPC("read", rpcPayload); e != nil {
		return
	}

	return m.parseRepositoryAssetsRPC(rpcResponse)
}

func (m *nexus) parseRepositoryAssetsRPC(rsp *rpcRsp) (e error) {
	var rspResult *rpcRspResult
	if e = json.Unmarshal(rsp.Result, &rspResult); e != nil {
		return
	}

	if !rspResult.Success {
		gLog.Warn().Msg("There is some errors in parsing RPC response. Api said that Result.Success is false!")
	}

	for _, obj := range rspResult.Data {
		switch obj["type"].(string) {
		case "folder":
			gQueue.newJob(&job{
				action:  jobActParseAsset,
				payload: []interface{}{m, obj["id"].(string)},
			})
		case "component":
			gQueue.newJob(&job{
				action:  jobActParseAsset,
				payload: []interface{}{m, obj["id"].(string)},
			})
		case "asset":
			if matched, _ := regexp.MatchString("((maven-metadata\\.xml)|\\.(pom|md5|sha1|sha256|sha512))$", obj["id"].(string)); matched {
				gLog.Debug().Msgf("The asset %s will be skipped!", obj["id"].(string))
				continue
			}

			asset := newRpcAsset(obj)
			m.mu.Lock()
			m.assetsCollection = append(m.assetsCollection, asset)
			assetsLen := len(m.assetsCollection)
			m.mu.Unlock()
			gLog.Debug().Int("count", assetsLen).Msgf("New asset collected! %s", asset.Name)
			gQueue.newJob(&job{
				action:  jobActGetAsset,
				payload: []interface{}{m, asset},
			})
		}
	}

	return
}

// !!
// TODO REFACTOR !!!
func (m *nexus) getRepositoryAssetInfo(asset NexusAsset2) (e error) {
	assetId, e := asset.getId()
	if e != nil {
		return
	}

	var rpcPayload = map[string]string{
		"0": assetId,
		"1": m.repository,
	}

	var rpcResponse *rpcRsp
	if rpcResponse, e = m.getRepositoryDataRPC("readAsset", rpcPayload); e != nil {
		return
	}

	if rpcResponse.ServerException != nil || len(rpcResponse.Message) != 0 {
		fmt.Println(rpcResponse.method + " " + string(rpcResponse.payload))
		fmt.Println(string(rpcResponse.Result))
		gLog.Error().Msg(rpcResponse.Message)
		gLog.Debug().Interface("ERR", rpcResponse.ServerException)
		return errRpcClientIntErr
	}

	return m.parseRepositoryAssetInfoRPC(asset, rpcResponse)
}

func (m *nexus) parseRepositoryAssetInfoRPC(asset NexusAsset2, rsp *rpcRsp) (e error) {
	var rspResult *rpcRspAssetResult
	if e = json.Unmarshal(rsp.Result, &rspResult); e != nil {
		fmt.Println(rsp.method + " " + string(rsp.payload))
		fmt.Println(string(rsp.Result))
		return
	}

	if !rspResult.Success {
		gLog.Warn().Msg("There is some errors in parsing RPC response. Api said that Result.Success is false!")
	}

	return asset.addAttributes(rspResult.Data)
}

/*func (m *ICQApi) parseChatMessagesResponse(chatId string, messages []*getHistoryRspResultMessage) (lastMsgId uint64, e error) {

	// if no messages - exit
	if len(messages) == 0 {
		return 0, e
	}

	lastMsgId = messages[len(messages)-1].MsgId
	for _, v := range messages {
		gDBQueue <- &job{
			action:  jobActCustomFunc,
			payload: []interface{}{v},
			payloadFunc: func(args []interface{}) error {
				var message = args[0].(*getHistoryRspResultMessage)
				return gMongoDB.UpdateOne("chats", bson.M{
					"aimId": chatId,
				}, bson.M{
					"$push": bson.M{
						"messages": &mongodb.CollectionChatsMessage{
							MsgId:  message.MsgId,
							Time:   time.Unix(message.Time, 0),
							Wid:    message.Wid,
							Sender: message.Chat.Sender,
							Text:   message.Text,
						},
					},
				})
			},
		}
	}

	return lastMsgId, e
}*/

func (m *nexus) incDownloadedAssets() {
	m.mu.Lock()
	m.dwnedAssets = m.dwnedAssets + 1
	m.mu.Unlock()
}

func (m *nexus) incUploadedAssets() {
	m.mu.Lock()
	m.upledAssets = m.upledAssets + 1
	m.mu.Unlock()
}

func (m *nexus) getRepositoryDataRPC(method string, data ...map[string]string) (rsp *rpcRsp, e error) {
	var payload []byte

	switch method {
	case "read":
		if payload, e = gRpc.newRpcJsonRequest(method, "coreui_Browse", data); e != nil {
			return
		}
	case "readAsset":
		var body = []string{
			data[0]["0"],
			data[0]["1"],
		}
		if payload, e = gRpc.newRpcJsonRequest(method, "coreui_Component", body); e != nil {
			return
		}
		// default:
		// 	if payload, e = gRpc.newRpcJsonRequest(method, "coreui_Component", data[0]); e != nil {
		// 		return
		// 	}
	}

	var rrl *url.URL
	if rrl, e = m.endpoint.Parse(gCli.String("rpc-endpoint")); e != nil {
		return
	}

	if e = gRpc.postRpcRequest(rrl.String(), payload, &rsp); e != nil {
		return
	}

	rsp.payload = payload
	rsp.method = method

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

func (m *nexus) downloadMissingAssetsRPC(assets []NexusAsset2, dstNexus *nexus) (e error) {
	var dwnListLen = len(assets)

	gLog.Info().Msgf("There are %d marked for downloading.", dwnListLen)

	if gCli.Bool("skip-download") {
		gLog.Warn().Msg("Skipping downloading because of --skip-download flag received!")
		return
	}

	for _, asset := range assets {
		gQueue.newJob(&job{
			action:  jobActDownloadAsset,
			payload: []interface{}{m, asset, dstNexus},
		})
	}
	return
}

func (m *nexus) downloadAssetRPC(asset NexusAsset2) (e error) {
	var dwnUrl string
	if dwnUrl, e = asset.getDownloadUrl(m.repository, m.endpoint); e != nil {
		return
	}

	var fd *os.File
	if fd, e = asset.getTemporaryFile(m.tempPath); e != nil {
		return
	}
	defer fd.Close()

	var rrl *url.URL
	if rrl, e = url.Parse(dwnUrl); e != nil {
		return
	}

	if e = m.api.getNexusFile(rrl.String(), fd); e != nil {
		return
		// gLog.Error().Err(e).Msgf("There is error while downloading asset. Asset %s will be skipped.", asset.getHumanReadbleName())
	}

	asset.setDownloaded()
	return
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

func (m *nexus) deleteAssetTemporaryFile(asset NexusAsset2) (e error) {
	var filepath string
	if filepath, e = asset.getTemporaryFilePath(m.tempPath); e != nil {
		return
	}

	if e = os.Remove(filepath); e != nil {
		gLog.Error().Msg("Could not delete file!")
		return
	}

	gLog.Debug().Msg("Uploaded assets has been successfully deleted from local filesystem.")
	asset.deleteAsset()
	runtime.GC()
	return
}

// if used it, do not forget about "Repair - Rebuild Maven repository metadata (maven-metadata.xml)" task
func (m *nexus) uploadAssetHttpFormatRPC(asset NexusAsset2) (e error) {
	var fd *os.File
	if fd, e = asset.isFileExists(m.tempPath); e != nil {
		return
	}
	defer fd.Close()

	var assetReqUri string
	if assetReqUri, e = asset.getDownloadUrl(m.repository, m.endpoint); e != nil {
		return
	}

	var rrl *url.URL
	if rrl, e = url.Parse(assetReqUri); e != nil {
		return
	}

	if e = m.api.putNexusFile(rrl.String(), fd); e != nil {
		return
	}

	return
}

func (m *nexus) uploadAssetRPC(asset NexusAsset2) (e error) {
	var fd *os.File
	if fd, e = asset.isFileExists(m.tempPath); e != nil {
		return
	}
	defer fd.Close()

	var apiForm = make(map[string]io.Reader)
	apiForm["asset0"] = fd
	var classifier, extension, groupId, artifactId, version string
	if classifier, e = asset.getClassifier(); e != nil {
		return
	}
	if extension, e = asset.getExtension(); e != nil {
		return
	}
	if groupId, e = asset.getGroupId(); e != nil {
		return
	}
	if artifactId, e = asset.getArtifactId(); e != nil {
		return
	}
	if version, e = asset.getVersion(); e != nil {
		return
	}

	apiForm["asset0.classifier"] = strings.NewReader(classifier)
	apiForm["asset0.extension"] = strings.NewReader(extension)
	apiForm["groupId"] = strings.NewReader(groupId)
	apiForm["artifactId"] = strings.NewReader(artifactId)
	apiForm["version"] = strings.NewReader(version)
	apiForm["generate-pom"] = strings.NewReader("on")

	var buffer *bytes.Buffer
	var contentType string
	if buffer, contentType, e = m.getNexusFileMeta(apiForm); e != nil {
		gLog.Error().Err(e).Str("filename", asset.getHumanReadbleName()).
			Msg("Could not get meta data for the asset's file.")
		return
	}

	var rrl *url.URL
	if rrl, e = m.endpoint.Parse("/service/rest/v1/components"); e != nil {
		return
	}

	var rgs = &url.Values{}
	rgs.Set("repository", m.repository)
	rrl.RawQuery = rgs.Encode()

	if e = m.api.postNexusFile(rrl.String(), buffer, contentType); e != nil {
		return
	}

	return
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
		defer file.Close()

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
		fileApiMeta["asset0.classifier"] = strings.NewReader(asset.Maven2.Classifier)
		fileApiMeta["asset0.extension"] = strings.NewReader(asset.Maven2.Extension)
		fileApiMeta["groupId"] = strings.NewReader(asset.Maven2.GroupID)
		fileApiMeta["artifactId"] = strings.NewReader(asset.Maven2.ArtifactID)
		fileApiMeta["version"] = strings.NewReader(asset.Maven2.Version)
		fileApiMeta["generate-pom"] = strings.NewReader("on")

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

		if e = m.api.postNexusFile(rrl.String(), body, contentType); e != nil {
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
