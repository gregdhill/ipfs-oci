package gantry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	contRef "github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/manifest"
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	files "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	ipfsPath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var ConfigAddress string

// ***************
// Image Reference
// ***************

type ipfsImgRef struct {
	api    iface.UnixfsAPI
	path   string
	logger *log.Logger
}

func (ref ipfsImgRef) Transport() types.ImageTransport {
	return is.Transport
}

func (ref ipfsImgRef) StringWithinTransport() string {
	return ""
}

func (ref ipfsImgRef) DockerReference() contRef.Named {
	return nil
}

func (ref ipfsImgRef) PolicyConfigurationIdentity() string {
	return ""
}

func (ref ipfsImgRef) PolicyConfigurationNamespaces() []string {
	return nil
}

func (ref ipfsImgRef) NewImage(ctx context.Context, sc *types.SystemContext) (types.ImageCloser, error) {
	src, err := ref.NewImageSource(ctx, sc)
	if err != nil {
		return nil, err
	}
	return image.FromSource(ctx, sc, src)
}

func (ref ipfsImgRef) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	ref.logger.Debug("Initializing IPFS source")
	node, err := ref.api.Get(ctx, ipfsPath.New(ref.path))
	if err != nil {
		return nil, err
	}

	file := node.(files.File)
	defer file.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(file)
	if err != nil {
		return nil, err
	}

	man := buf.Bytes()
	manType := manifest.GuessMIMEType(man)

	return ipfsImgSrc{
		api:     ref.api,
		man:     man,
		manType: manType,
		logger:  ref.logger,
	}, nil
}

func (ref ipfsImgRef) NewImageDestination(ctx context.Context, sys *types.SystemContext) (types.ImageDestination, error) {
	ref.logger.Debug("Initializing IPFS destination")
	return ipfsImgDst{
		api:    ref.api,
		logger: ref.logger,
	}, nil
}

func (ref ipfsImgRef) DeleteImage(ctx context.Context, sys *types.SystemContext) error {
	return nil
}

// ************
// Image Source
// ************

type ipfsImgSrc struct {
	api     iface.UnixfsAPI
	man     []byte
	manType string
	logger  *log.Logger
}

func (src ipfsImgSrc) Reference() types.ImageReference {
	return ipfsImgRef{}
}

func (src ipfsImgSrc) Close() error {
	return nil
}

func (src ipfsImgSrc) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		return nil, "", errors.Errorf("instanceDigest is nil")
	}
	return src.man, src.manType, nil
}

func (src ipfsImgSrc) GetBlob(ctx context.Context, blob types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if len(blob.URLs) < 1 {
		return nil, 0, fmt.Errorf("nothing in BlobInfo.URLs")
	}

	src.logger.Debugf("Fetching blob for %s", blob.Digest)
	rawCID := blob.URLs[0]
	node, err := src.api.Get(ctx, ipfsPath.New(rawCID))
	if err != nil {
		return nil, 0, err
	}

	reader := node.(files.File)
	size, err := reader.Size()
	return reader, size, err
}

func (src ipfsImgSrc) HasThreadSafeGetBlob() bool {
	return true
}

func (src ipfsImgSrc) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	if instanceDigest != nil {
		return nil, errors.Errorf("instanceDigest is nil")
	}
	return nil, nil
}

func (src ipfsImgSrc) LayerInfosForCopy(ctx context.Context) ([]types.BlobInfo, error) {
	return nil, nil
}

// *****************
// Image Destination
// *****************

type ipfsImgDst struct {
	api    iface.UnixfsAPI
	logger *log.Logger
}

func (dst ipfsImgDst) Reference() types.ImageReference {
	return ipfsImgRef{}
}

func (dst ipfsImgDst) Close() error {
	return nil
}

func (dst ipfsImgDst) SupportedManifestMIMETypes() []string {
	return manifest.DefaultRequestedManifestMIMETypes
}

func (dst ipfsImgDst) SupportsSignatures(ctx context.Context) error {
	return nil
}

func (dst ipfsImgDst) DesiredLayerCompression() types.LayerCompression {
	return 0
}

func (dst ipfsImgDst) AcceptsForeignLayerURLs() bool {
	return true
}

func (dst ipfsImgDst) MustMatchRuntimeOS() bool {
	return false
}

func (dst ipfsImgDst) IgnoresEmbeddedDockerReference() bool {
	return false
}

func (dst ipfsImgDst) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	img, err := ioutil.ReadAll(stream)
	if err != nil {
		return inputInfo, err
	}

	pr, err := dst.api.Add(ctx, files.NewBytesFile(img))
	if err != nil {
		return inputInfo, err
	}

	addr := pr.Cid().String()
	if isConfig {
		dst.logger.Debugf("Setting config cid %s for %s", addr, inputInfo.Digest)
		ConfigAddress = addr
	}

	inputInfo.URLs = []string{addr}
	return inputInfo, nil
}

func (dst ipfsImgDst) HasThreadSafePutBlob() bool {
	return false
}

func (dst ipfsImgDst) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	return false, types.BlobInfo{}, nil
}

func (dst ipfsImgDst) PutManifest(ctx context.Context, man []byte) error {
	manSchema, err := manifest.Schema2FromManifest(man)
	manSchema.ConfigDescriptor.URLs = []string{ConfigAddress}
	man, err = manSchema.Serialize()
	if err != nil {
		return err
	}

	pr, err := dst.api.Add(ctx, files.NewBytesFile(man))
	if err != nil {
		return err
	}

	fmt.Println(pr.Cid().String())
	return nil
}

func (dst ipfsImgDst) PutSignatures(ctx context.Context, signatures [][]byte) error {
	return nil
}

func (dst ipfsImgDst) Commit(ctx context.Context) error {
	return nil
}
