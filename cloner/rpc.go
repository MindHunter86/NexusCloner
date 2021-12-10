package cloner

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"strings"
)

type (
	rpcRsp struct {
		Tid    int                        `json:"tid,omitempty"`
		Action string                     `json:"action,omitempty"`
		Method string                     `json:"method,omitempty"`
		Result map[string]json.RawMessage `json:"result,omitempty"`
	}
	rpcRspResult struct {
		Success bool                     `json:"success,omitempty"`
		Data    []map[string]interface{} `json:"data,omitempty"` // dynamic field in struct !!
	}
	rpcRspAssetResult struct {
		Success bool                   `json:"success,omitempty"`
		Data    map[string]interface{} `json:"data,omitempty"` // dynamic field in struct !!
	}
	rpcTree struct {
		Id   string `json:"id,omitempty"`
		Text string `json:"text,omitempty"`
		Type string `json:"type,omitempty"`
	}
	rpcComponent struct {
		Id     string `json:"id,omitempty"`
		Name   string `json:"text,omitempty"`
		Format string `json:"type,omitempty"`
	}
	rpcAsset struct {
		Id          string         `json:"id,omitempty"`
		Name        string         `json:"name,omitempty"`
		Format      string         `json:"format,omitempty"`
		Attributes  *rpcAssetAttrs `json:"attributes,omitempty"`
		DownloadUrl string         `json:"downloadUrl,omitempty"`
	}
	rpcAssetAttrs struct {
		Checksum *rpcAssetAttrsChecksum `json:"checksum,omitempty"`
		Maven2   *rpcAssetAttrsMaven2   `json:"maven2,omitempty"`
	}
	rpcAssetAttrsChecksum struct {
		Md5    string `json:"md5,omitempty"`
		Sha1   string `json:"sha1,omitempty"`
		Sha256 string `json:"sha256,omitempty"`
		Sha512 string `json:"sha512,omitempty"`
	}
	rpcAssetAttrsMaven2 struct {
		Extension  string `json:"extension,omitempty"`
		GroupId    string `json:"groupId,omitempty"`
		ArtifactId string `json:"artifactId,omitempty"`
		Version    string `json:"version,omitempty"`
	}
)

type (
	rpcRequest struct {
		Action string      `json:"action"`
		Data   interface{} `json:"data"`
		Method string      `json:"method"`
		Tid    int         `json:"tid"`
		Type   string      `json:"type"`
	}
)

var errRpcAssetPanic = errors.New("Panic caught in the asset! The asset was invalid and will be skipped.")
var errRpcAssetEmpty = errors.New("The asset is invalid because of empty or has empty attributes!")

func newRpcAsset(rpcObject map[string]interface{}) *rpcAsset {
	return &rpcAsset{
		Id:   rpcObject["assetId"].(string),
		Name: rpcObject["id"].(string),
	}
}

func (m *rpcAsset) addAttributes(rpcObject map[string]interface{}) (e error) {
	attrPayload := rpcObject["attributes"].(json.RawMessage)

	var assetAttrs *rpcAssetAttrs
	if e = json.Unmarshal(attrPayload, &assetAttrs); e != nil {
		return
	}

	m.Attributes = assetAttrs
	gLog.Debug().Msgf("filename %s : sha1 - %s", m.Name, assetAttrs.Checksum.Sha1)
	return
}

func (m *rpcAsset) catchPanic(err *error) {
	if recover() != nil {
		*err = errRpcAssetPanic
	}
}

func (m *rpcAsset) getDownloadUrl() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.DownloadUrl) != 0 {
		data = m.DownloadUrl
	}

	return
}

func (m *rpcAsset) getExtension() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.Attributes.Maven2.Extension) != 0 {
		data = m.Attributes.Maven2.Extension
	}

	return
}

func (m *rpcAsset) getGroupId() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.Attributes.Maven2.GroupId) != 0 {
		data = m.Attributes.Maven2.GroupId
	}

	return
}

func (m *rpcAsset) getArtifactId() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.Attributes.Maven2.ArtifactId) != 0 {
		data = m.Attributes.Maven2.ArtifactId
	}

	return
}
func (m *rpcAsset) getVersion() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.Attributes.Maven2.Version) != 0 {
		data = m.Attributes.Maven2.Version
	}

	return
}

func (m *rpcAsset) getId() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.Id) != 0 {
		data = m.Id
	}

	return
}

func (m *rpcAsset) getHumanReadbleName() string {
	return strings.ReplaceAll(m.Name, "/", "_")
}

func (m *rpcAsset) isFileExists(tmpdir string) (file *os.File, e error) {
	var filename = path.Base(m.Name)
	if file, e = os.OpenFile(tmpdir+"/"+filename, os.O_RDONLY, 0600); e != nil {
		if !errors.Is(e, os.ErrNotExist) {
			return
		}

		gLog.Error().Err(e).Str("filename", filename).
			Msg("Internal error! The asset file not found. Is download ok? The asset will be skipped.")
		return
	}

	return
}

// TODO
// OPTIMIZE - https://pkg.go.dev/os@go1.17.2#OpenFile
// !! Note - returned FD must be closed!!
func (m *rpcAsset) getTemporaryFile(tmpdir string) (file *os.File, e error) {
	var filename = path.Base(m.Name)

	if file, e = os.OpenFile(tmpdir+"/"+filename, os.O_RDWR|os.O_CREATE, 0600); e != nil {
		if !errors.Is(e, os.ErrNotExist) {
			return
		}

		gLog.Warn().Err(e).Str("filename", filename).
			Msg("Given filename was found. It will be rewritten in the next iterations.")
		return
	}

	return os.Create(tmpdir + "/" + filename)
}

// func (m *rpcAsset) downloadAsset() error                      {}
// func (m *rpcAsset) getBinaryFile() (*os.File, error)          {}
