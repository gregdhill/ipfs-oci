package image

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
	iface "github.com/ipfs/interface-go-ipfs-core"
	log "github.com/sirupsen/logrus"
)

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
func PushImage(ctx context.Context, logger *log.Entry, store storage.Store, api iface.CoreAPI, image string) error {
	systemContext, policyContext, err := getContexts(store)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	dstRef := NewImageReference(logger, api.Unixfs(), "")
	if err != nil {
		return err
	}

	srcRef, _, err := util.FindImage(store, "", systemContext, image)
	if err != nil {
		return err
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
func PullImage(ctx context.Context, logger *log.Entry, store storage.Store, api iface.CoreAPI, cid, image string) error {
	systemContext, policyContext, err := getContexts(store)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	srcRef := NewImageReference(logger, api.Unixfs(), cid)
	dstRef, err := is.Transport.ParseStoreReference(store, image)
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
