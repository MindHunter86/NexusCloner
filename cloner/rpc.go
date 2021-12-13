package cloner

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
)

type (
	rpcRsp struct {
		Tid             int                    `json:"tid,omitempty"`
		Action          string                 `json:"action,omitempty"`
		Method          string                 `json:"method,omitempty"`
		Result          json.RawMessage        `json:"result,omitempty"`
		Message         string                 `json:"message,omitempty"`
		ServerException map[string]interface{} `json:"serverException,momitempty"`

		method, action string
		payload        []byte
	}
	rpcRspResult struct {
		Success bool                     `json:"success,omitempty"`
		Data    []map[string]interface{} `json:"data,omitempty"` // dynamic field in struct !!
	}
	rpcRspAssetResult struct {
		Success bool                       `json:"success,omitempty"`
		Data    map[string]json.RawMessage `json:"data,omitempty"` // dynamic field in struct !!
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
		ContentType string         `json:"contentType,omitempty"`

		dwnedFd      *os.File
		dwnedSuccess bool
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
		Extension   string `json:"extension,omitempty"`
		GroupId     string `json:"groupId,omitempty"`
		ArtifactId  string `json:"artifactId,omitempty"`
		Version     string `json:"version,omitempty"`
		Classifier  string `json:"classifier,omitempty"`
		BaseVersion string `json:"baseVersion,omitempty"`
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

func (m *rpcAsset) addAttributes(rpcObject map[string]json.RawMessage) (e error) {
	// attrPayload := rpcObject["attributes"]

	var assetAttrs *rpcAssetAttrs
	if e = json.Unmarshal(rpcObject["attributes"], &assetAttrs); e != nil {
		return
	}

	// m.Attributes = assetAttrs
	// m.Attributes = &attrPayload
	// gLog.Debug().Msgf("filename %s : sha1 - %s", m.Name, attrPayload.Checksum.Sha1)
	m.Attributes = assetAttrs
	gLog.Debug().Msgf("filename %s : sha1 - %s", m.Name, m.Attributes.Checksum.Sha1)
	return
}

func (m *rpcAsset) catchPanic(err *error) {
	if recover() != nil {
		*err = errRpcAssetPanic
	}
}

func (m *rpcAsset) getDownloadUrl(reponame string, baseurl *url.URL) (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.DownloadUrl) != 0 {
		data = m.DownloadUrl
	} else {
		var drl *url.URL
		if drl, e = baseurl.Parse(fmt.Sprintf("/repository/%s/%s", reponame, m.Name)); e != nil {
			return
		}

		if gCli.Bool("maven-snapshots") {
			buf := strings.Split(drl.String(), "/")
			bufLen := len(buf)
			gLog.Debug().Msg("[DOWNLOADURL] parsed version - " + buf[bufLen-2])

			data = strings.ReplaceAll(drl.String(), buf[bufLen-2]+"/", "")
		}

		data = drl.String()
		gLog.Debug().Msg("Download URL - " + data)
		// https://HOST/repository/maven-ums/ru/mts/tvhouse/tvh-core/1.0-M1-20181002.132713-41/tvh-core-1.0-M1-20181002.132713-41-sources.log
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

func (m *rpcAsset) getClassifier() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.Attributes.Maven2.Classifier) != 0 {
		data = m.Attributes.Maven2.Classifier
	}

	return
}

func (m *rpcAsset) getBaseVersion() (data string, e error) {
	defer m.catchPanic(&e)

	if len(m.Attributes.Maven2.BaseVersion) != 0 {
		data = m.Attributes.Maven2.BaseVersion
	}

	return
}

func (m *rpcAsset) getHashes() (data map[string]string, e error) {
	defer m.catchPanic(&e)
	data = make(map[string]string)

	if len(m.Attributes.Checksum.Md5) != 0 {
		data["md5"] = m.Attributes.Checksum.Md5
	}

	if len(m.Attributes.Checksum.Sha1) != 0 {
		data["sha1"] = m.Attributes.Checksum.Sha1
	}

	if len(m.Attributes.Checksum.Sha256) != 0 {
		data["sha256"] = m.Attributes.Checksum.Sha256
	}

	if len(m.Attributes.Checksum.Sha512) != 0 {
		data["sha512"] = m.Attributes.Checksum.Sha512
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

func (m *rpcAsset) getTemporaryFilePath(tmpdir string) (filepath string, e error) {
	filename := path.Base(m.Name)
	filepath = tmpdir + "/" + filename
	if _, e = os.Stat(filepath); e != nil {
		if errors.Is(e, os.ErrNotExist) {
			gLog.Error().Err(e).Msg("There is internal error in asset temporary file processing! Given asset was not found!")
			return
		}
		return
	}
	return
}

// TODO
// OPTIMIZE - https://pkg.go.dev/os@go1.17.2#OpenFile
// !! Note - returned FD must be closed!!
func (m *rpcAsset) getTemporaryFile(tmpdir string) (file *os.File, e error) {
	var filename = path.Base(m.Name)
	defer func() { m.dwnedFd = file }()

	if file, e = os.OpenFile(tmpdir+"/"+filename, os.O_RDWR|os.O_CREATE, 0600); e != nil {
		if !errors.Is(e, os.ErrNotExist) {
			return
		}

		gLog.Warn().Err(e).Str("filename", filename).
			Msg("Given filename was found. It will be rewritten in the next iterations.")
		return
	}

	file, e = os.Create(tmpdir + "/" + filename)
	return
}

// func (m *rpcAsset) downloadAsset() error                      {}
// func (m *rpcAsset) getBinaryFile() (*os.File, error)          {}

// struct compressing
func (m *rpcAsset) compressAsset() ([]byte, error) {
	return m.encodeAssetToBytes()
}

func restoreCompressedAsset(slice []byte) (NexusAsset2, error) {
	asset := &rpcAsset{}
	return asset.decompressAssetBytes(slice)
}

func (m *rpcAsset) encodeAssetToBytes() ([]byte, error) {
	var e error
	buffer := bytes.Buffer{}
	encoder := gob.NewEncoder(&buffer)

	if e = encoder.Encode(m); e != nil {
		return nil, e
	}

	gLog.Debug().Msgf("uncompressed size (bytes): %d", len(buffer.Bytes()))
	return m.compressAssetBytes(buffer.Bytes())
}

func (m *rpcAsset) compressAssetBytes(slice []byte) ([]byte, error) {
	var e error
	buffer := bytes.Buffer{}
	gzipWriter := gzip.NewWriter(&buffer)
	defer gzipWriter.Close()

	if _, e := gzipWriter.Write(slice); e != nil {
		return nil, e
	}

	gLog.Debug().Msgf("compressed size (bytes): %d", len(buffer.Bytes()))
	return buffer.Bytes(), e
}

func (m *rpcAsset) decompressAssetBytes(slice []byte) (NexusAsset2, error) {
	var e error
	var encodedSlice []byte

	var gzipReader *gzip.Reader
	if gzipReader, e = gzip.NewReader(bytes.NewReader(slice)); e != nil {
		return nil, e
	}
	defer gzipReader.Close()

	if encodedSlice, e = ioutil.ReadAll(gzipReader); e != nil {
		return nil, e
	}

	gLog.Debug().Msgf("uncompressed size (bytes): %d", len(encodedSlice))
	return m.decodeAssetFromBytes(encodedSlice)
}

func (m *rpcAsset) decodeAssetFromBytes(slice []byte) (NexusAsset2, error) {
	var e error
	decoder := gob.NewDecoder(bytes.NewReader(slice))

	// Maybe it's bad idea and before decoding we need to create NexusAsset2 object.
	// !! This block neew debugging and test
	if e = decoder.Decode(&m); e != nil {
		return nil, e
	}

	return m, e
}

// filesystem working
func (m *rpcAsset) getAssetFd() *os.File { return m.dwnedFd }
func (m *rpcAsset) isDownloaded() bool   { return m.dwnedSuccess }
func (m *rpcAsset) setDownloaded()       { m.dwnedSuccess = true }

func (m *rpcAsset) deleteAsset() {
	// rewrite current struct with empty object for memory free
	// it will be freed by the GC in him next iteration
	*m = rpcAsset{}
	return
}
