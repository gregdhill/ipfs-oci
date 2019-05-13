package gantry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/buildah/pkg/unshare"
	"github.com/containers/buildah/util"
	cp "github.com/containers/image/copy"
	"github.com/containers/image/signature"
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	log "github.com/sirupsen/logrus"
)

func loadConfig(path string) (*config.Config, error) {
	return fsrepo.ConfigAt(path)
}

func setupLocal(ctx context.Context) (*core.IpfsNode, error) {
	path, err := fsrepo.BestKnownPath()
	if err != nil {
		return nil, err
	}

	pluginpath := filepath.Join(path, "plugins")
	plugins, err := loader.NewPluginLoader(pluginpath)
	if err != nil {
		return nil, err
	}
	if err = plugins.Initialize(); err != nil {
		return nil, err
	}
	if err = plugins.Inject(); err != nil {
		return nil, err
	}

	repo, err := fsrepo.Open(path)
	if err != nil {
		return nil, err
	}

	node, err := core.NewNode(ctx, &core.BuildCfg{
		Repo: repo,
	})
	return node, err
}

func getContexts(store storage.Store) (*types.SystemContext, *signature.PolicyContext, error) {
	systemContext := &types.SystemContext{}
	if store != nil {
		if systemContext.BlobInfoCacheDir == "" {
			systemContext.BlobInfoCacheDir = filepath.Join(store.GraphRoot(), "cache")
		}
		if systemContext.SystemRegistriesConfPath == "" && unshare.IsRootless() {
			userRegistriesFile := filepath.Join(store.GraphRoot(), "registries.conf")
			if _, err := os.Stat(userRegistriesFile); err == nil {
				systemContext.SystemRegistriesConfPath = userRegistriesFile
			}
		}
	}

	policy, err := signature.DefaultPolicy(systemContext)
	if err != nil {
		return nil, nil, err
	}

	policyContext, err := signature.NewPolicyContext(policy)
	return systemContext, policyContext, err
}

// GetStore returns the environment's pre-existing store
func GetStore() (storage.Store, error) {
	storeOptions, err := storage.DefaultStoreOptions(unshare.IsRootless(), unshare.GetRootlessUID())
	if err != nil {
		return nil, err
	}

	if os.Geteuid() != 0 && storeOptions.GraphDriverName != "vfs" {
		return nil, err
	}

	store, err := storage.GetStore(storeOptions)
	if err != nil {
		return nil, err
	} else if store != nil {
		is.Transport.SetStore(store)
	}

	return store, nil
}

// PushImage to IPFS
func PushImage(logger *log.Logger, store storage.Store, imageName string) error {
	systemContext, policyContext, err := getContexts(store)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	ctx := context.Background()
	node, err := setupLocal(ctx)
	if err != nil {
		return err
	}

	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}

	srcRef, _, err := util.FindImage(store, "", systemContext, imageName)
	if err != nil {
		return err
	}

	dstRef := ipfsImgRef{
		api:    api.Unixfs(),
		logger: logger,
	}

	options := cp.Options{
		SourceCtx:      systemContext,
		DestinationCtx: systemContext,
		ReportWriter:   logger.WriterLevel(log.DebugLevel),
	}

	_, err = cp.Image(ctx, policyContext, dstRef, srcRef, &options)
	if err != nil {
		return err
	}

	return nil
}

// PullImage from IPFS
func PullImage(logger *log.Logger, store storage.Store, cid, target string) error {
	systemContext, policyContext, err := getContexts(store)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	ctx := context.Background()
	node, err := setupLocal(ctx)
	if err != nil {
		return err
	}

	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}

	// img *storage.Image
	srcRef := ipfsImgRef{
		api:    api.Unixfs(),
		path:   cid,
		logger: logger,
	}

	dstRef, err := is.Transport.ParseStoreReference(store, target)
	if err != nil {
		return err
	}

	// maybeCachedDestRef := types.ImageReference(dstRef)
	// cachedRef, err := blobcache.NewBlobCache(dstRef, "/home/greg/go/src/github.com/gregdhill/launch/store", types.PreserveOriginal)
	// maybeCachedDestRef = cachedRef

	options := cp.Options{
		SourceCtx:      systemContext,
		DestinationCtx: systemContext,
		ReportWriter:   logger.WriterLevel(log.DebugLevel),
	}

	_, err = cp.Image(ctx, policyContext, dstRef, srcRef, &options)
	if err != nil {
		return err
	}

	taggedImg, err := is.Transport.GetStoreImage(store, dstRef)
	fmt.Println(taggedImg.ID)
	return nil
}
