package s3gen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// contentHash computes a SHA256 hash of the resource's content.
// This is used for deduplicating shared assets.
func contentHash(res *Resource) string {
	data, err := os.ReadFile(res.FullPath)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ContentHashShort returns the first 8 characters of the content hash.
// This is typically used for generating unique asset directory names.
func ContentHashShort(res *Resource) string {
	hash := contentHash(res)
	if len(hash) >= 8 {
		return hash[:8]
	}
	return hash
}

// DefaultAssetHandler provides default asset handling for content resources.
// It copies assets to the same directory as the output HTML, or to a shared
// assets directory for parametric pages.
type DefaultAssetHandler struct{}

// HandleAssets implements AssetAwareRule for default asset handling.
func (h *DefaultAssetHandler) HandleAssets(site *Site, res *Resource, assets []*Resource) ([]AssetMapping, error) {
	var mappings []AssetMapping

	for _, asset := range assets {
		var destPath string

		if res.IsParametric {
			// Parametric pages: assets go to shared folder with content-hash
			hash := ContentHashShort(asset)
			destPath = filepath.Join(site.SharedAssetsDir, hash, filepath.Base(asset.FullPath))
		} else {
			// Non-parametric: co-locate with output
			// Determine output directory based on resource path
			respath := res.RelPath()
			if respath == "" {
				continue
			}

			ext := filepath.Ext(respath)
			rem := respath[:len(respath)-len(ext)]

			var destDir string
			if res.IsIndex {
				destDir = filepath.Dir(respath)
			} else {
				destDir = rem
			}

			destPath = filepath.Join(destDir, filepath.Base(asset.FullPath))
		}

		mappings = append(mappings, AssetMapping{
			Source: asset,
			Dest:   destPath,
			Action: AssetCopy,
		})
	}

	return mappings, nil
}

// GetAssetURL returns the URL for an asset relative to the current resource.
// This can be used in templates to reference co-located assets.
func GetAssetURL(site *Site, res *Resource, filename string) string {
	// Check if the file is in the resource's assets
	for _, asset := range res.Assets {
		if filepath.Base(asset.FullPath) == filename {
			if res.IsParametric {
				// Shared assets path
				hash := ContentHashShort(asset)
				return site.PathPrefix + "/" + site.SharedAssetsDir + "/" + hash + "/" + filename
			}
			// Co-located path - relative URL
			return "./" + filename
		}
	}
	// Fallback to static folder
	return site.PathPrefix + "/static/" + filename
}
