package image

import (
	"context"
	"path/filepath"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	iface "github.com/ipfs/interface-go-ipfs-core"
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

	return core.NewNode(ctx, &core.BuildCfg{
		Repo: repo,
	})
}

func ResolveName(ctx context.Context, api iface.CoreAPI, name string) (string, error) {
	ipfsPath, err := api.Name().Resolve(ctx, name)
	if err != nil {
		return "", err
	}
	return ipfsPath.String(), nil
}

func GetAPI(ctx context.Context) (iface.CoreAPI, error) {
	node, err := setupLocal(ctx)
	if err != nil {
		return nil, err
	}

	return coreapi.NewCoreAPI(node)
}
