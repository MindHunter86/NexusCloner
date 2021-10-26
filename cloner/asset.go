package cloner

import (
	"errors"
	"os"
	"path"
)

type (
	NexusAssetsCollection struct {
		Items             []*NexusAsset `json:"items,omitempty"`
		ContinuationToken string        `json:"continuationToken,omitempty"`
	}

	NexusAsset struct {
		DownloadURL  string              `json:"downloadUrl,omitempty"`
		Path         string              `json:"path,omitempty"`
		ID           string              `json:"id,omitempty"`
		Repository   string              `json:"repository,omitempty"`
		Format       string              `json:"format,omitempty"`
		Checksum     *NexusAssetChecksum `json:"checksum,omitempty"`
		ContentType  string              `json:"contentType,omitempty"`
		LastModified string              `json:"lastModified,omitempty"`
		BlobCreated  string              `json:"blobCreated,omitempty"`
		LastDownload string              `json:"lastDownloaded,omitempty"`
		Maven2       *NexuAssetMaven2    `json:"maven2,omitempty"`
	}

	NexusAssetChecksum struct {
		Sha1   string `json:"sha1,omitempty"`
		Sha512 string `json:"sha512,omitempty"`
		Sha256 string `json:"sha256,omitempty"`
		Md5    string `json:"md5,omitempty"`
	}

	NexuAssetMaven2 struct {
		Extension  string `json:"extension,omitempty"`
		GroupID    string `json:"groupId,omitempty"`
		ArtifactID string `json:"artifactId,omitempty"`
		Version    string `json:"version,omitempty"`
	}
)

// TODO
// OPTIMIZE - https://pkg.go.dev/os@go1.17.2#OpenFile
// !! Note - returned FD must be closed!!
func (m *NexusAsset) getTemporaryFile(tmpdir string) (file *os.File, e error) {
	var filename = path.Base(m.Path)

	if file, e = os.OpenFile(tmpdir+"/"+filename, os.O_RDWR|os.O_CREATE, 0600); e != nil {
		if !errors.Is(e, os.ErrNotExist) {
			return
		}

		gLog.Warn().Err(e).Str("filename", filename).Msg("Given filename was found. It will be rewritten in the next iterations.")
		return
	}

	return os.Create(tmpdir + "/" + filename)
}
