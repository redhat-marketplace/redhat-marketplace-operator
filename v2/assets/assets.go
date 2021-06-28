package assets

import (
	"bytes"
	"embed"
	"io"
)

//go:embed *
var assets embed.FS

func MustReadFileAsset(filename string) []byte {
	bts, err := assets.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return bts
}

func MustAssetReader(asset string) io.Reader {
	return bytes.NewReader(MustReadFileAsset(asset))
}
