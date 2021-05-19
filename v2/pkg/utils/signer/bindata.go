// Code generated for package signer by go-bindata DO NOT EDIT. (@generated)
// sources:
// ../../../assets/signer/ca.pem
package signer

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _signerCaPem = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x94\x94\xcb\xb2\xaa\x38\x14\x86\xe7\x3c\x45\xcf\xad\x2e\xc4\x8d\x28\xc3\x15\x12\x2e\x6a\xd0\x40\x00\x61\x06\xe8\x06\x11\x50\x40\x6e\x3e\x7d\x97\x7b\x74\xea\xf4\xe9\x41\x67\xb8\xea\x4b\xd5\x97\xfc\xc9\xff\xf7\x67\x21\x62\x58\xf6\x5f\x1a\x71\xb8\xa5\x5b\x1a\x70\xf2\x33\x15\xa8\x65\xe1\x88\x6b\x1a\xdc\x57\x19\x8c\x16\x82\xcc\xda\xc1\x3e\xdd\x4f\xc3\xf9\x95\x4e\xf5\x4c\x61\x69\x68\x6e\x63\xb8\x56\xf2\x85\x19\x41\xda\xe8\x01\x25\xd9\xa4\xbd\x61\x87\x32\xdb\x17\x10\x84\x1c\x4a\x9f\x53\xd6\x8d\x1a\x0b\xb1\xcf\x98\x85\x61\x77\x64\x6f\xc2\x28\xc8\x06\x48\x1e\x41\xe3\x68\x7a\x95\xde\x45\x41\x59\xc7\x98\x50\x0a\x8f\x9f\xb9\x96\x8d\x58\x70\xbd\x9d\x4d\xd9\x38\x6a\xd9\xcf\xe6\x03\x06\xdb\x75\xc9\x72\xb4\x50\x6a\xd3\x82\x8c\x94\x93\x89\xf2\xfb\x68\x73\xb2\x0e\xf2\xec\x44\xdf\x30\x53\x0c\x12\xe5\x6c\x3a\x72\x90\x04\xca\xcb\xf8\x57\xa5\xff\x6b\x24\xfc\xae\xf4\x5f\x46\x59\x46\x6e\xbf\xdf\x07\x30\x0f\x04\x90\x2d\x84\x47\xf8\x00\x7b\x78\x58\x08\x18\x7e\x5e\x63\x5b\x7a\xdd\xa6\xda\x0d\x56\x56\x93\x40\x33\x71\xe4\xad\x62\x2e\x7b\x2e\x2e\xa0\xbf\xf8\x43\x7f\x7f\x3a\x6e\x55\x9e\x65\xd0\x85\x1c\xba\xfc\x7b\xa4\xf1\x70\x75\xb9\xe4\xb1\x78\x9f\x6d\xa5\xa3\x5d\x03\xe6\x61\x08\xf5\xec\x2d\xc3\x59\xf3\xcb\xf4\x52\x87\xd5\x81\xb5\x43\x58\x16\xe6\x88\xa5\xf6\x60\x2f\xaa\xac\x81\xd7\x4e\x68\xed\x4d\x00\x60\x14\xb0\x11\xc3\xfa\x75\x20\xb9\xfa\xf4\x57\x5d\xde\x0c\xd3\xe3\x2d\x5e\xa2\x5c\x5e\x14\xd1\x76\x86\xfd\x42\xbc\x76\xdd\x69\x2a\xae\xc3\xba\xb6\xad\x52\xdb\xc6\xa6\x3f\x14\x8c\x08\xae\xcb\x8a\x79\xfe\xf2\x87\xc6\x36\x42\xdd\xdd\xd4\x96\x22\xb7\xa7\xb3\x2f\x72\xb3\xde\x3f\xe0\xa5\x3a\xd8\xd4\x5d\xcb\x34\xa2\x8d\x3c\x04\xe7\xd9\xdf\x89\x63\x3a\x78\xc6\x02\x9e\x62\xbc\x59\x33\x81\x58\xfc\x94\x1e\xc9\xb2\xe7\x69\xf1\xb8\x3e\x82\x38\x79\xb5\xaa\x13\x68\xab\x94\x6d\x1d\x35\x27\x41\xbf\x08\x76\x74\x9c\x0e\xe2\x2e\xf9\x72\xef\xf8\xac\x1e\x34\xde\xce\x45\xed\x5c\xd6\x8a\x12\xc5\x42\x8b\x93\xb0\x4f\x5a\x37\x36\x02\xca\xbf\x8b\x22\x8b\xc4\x40\xc6\xa6\x97\xa5\x4c\x4c\x7b\x74\xce\xb6\x74\xa6\x81\x53\x43\x46\xd1\xe7\xa8\x1e\x46\x47\x8a\x96\x9f\xd4\x2e\x38\x63\x81\x80\x10\x5f\x9c\x7a\x13\x65\x2b\xf9\x59\xc9\x89\x64\xe9\xad\x79\x18\xb9\x47\x7b\xb4\x48\x38\x7c\x7f\xde\x86\xe9\x52\x62\x60\x08\xb2\x3f\xb0\xc2\x2f\x30\xfd\x81\x1d\x4a\x10\x07\x0c\xcc\x14\xff\xf4\x0f\x3e\xb1\x03\x43\xc9\x5a\x31\xf0\xaa\xd5\x3c\xc1\xdc\xac\x91\xdb\xa8\x78\xc3\x5a\xe7\xe1\x74\xf5\x15\x3b\x8a\xeb\x6b\xa0\x85\xf6\xf6\xf2\x5c\x05\x5f\x70\xfb\x7a\x95\xdf\xbc\x92\x82\x4b\x77\x8d\xc3\xb8\x52\x1d\x32\x58\x53\xfc\x08\x78\x3f\xee\x7b\xa1\xf2\xc3\xab\x58\x89\x51\x93\xf0\x88\x06\x4a\x73\x30\x1c\x11\x55\x1e\xbb\x59\x4f\x59\xbc\x44\x4e\x4e\xc0\x66\x48\x75\x57\x8f\x66\x2c\xa3\xde\x97\x67\xf9\x40\x31\x4a\xfc\xb3\xf2\xad\x47\x26\x11\x22\xbb\xbc\x2b\xf7\x63\xa1\xa7\x79\xb7\xe7\xeb\x93\xeb\xa8\x97\x41\x76\x0d\xfa\xd6\x89\xa8\x76\x63\x6b\xfa\xdb\xeb\xc9\x1e\x38\xc3\xeb\x3c\x2a\x4d\xb1\x52\x4a\x8b\x6e\xce\x45\xbe\x97\xbf\xc7\xa8\x13\x5e\x4b\x58\x1c\x90\x62\xdc\xcf\x12\x0d\x1f\x71\xaf\x9c\x32\x69\x1b\x9c\x4b\xfa\x16\x9b\x27\x0f\xda\xdd\x2b\xe2\xba\x1b\x15\xe6\x3a\xec\xf9\xac\xfb\xd7\x7a\x4f\xa6\xcc\x5f\x1e\xfb\xce\x1d\x56\xc6\x97\x30\x33\xe3\xa5\xf7\x3d\x91\xde\x3b\x96\xd6\x8b\x63\xa3\x48\x67\xde\xf7\x1c\xa6\x44\xcd\x9b\x34\xb2\x16\x8f\x14\x44\xf5\x36\x2e\xaa\x57\xbf\xf6\xab\x3b\x2d\xd3\x4d\xba\x4f\xa6\xba\xe9\x8c\x40\x95\x85\xe4\xb0\x62\xef\x72\x7d\x53\xe6\x40\x16\x7e\x4a\x8a\xd8\xf8\xdf\xc5\xf5\x4f\x00\x00\x00\xff\xff\x87\xc1\x51\xe6\xd5\x04\x00\x00")

func signerCaPemBytes() ([]byte, error) {
	return bindataRead(
		_signerCaPem,
		"signer/ca.pem",
	)
}

func signerCaPem() (*asset, error) {
	bytes, err := signerCaPemBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "signer/ca.pem", size: 1237, mode: os.FileMode(420), modTime: time.Unix(1621362653, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"signer/ca.pem": signerCaPem,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"signer": &bintree{nil, map[string]*bintree{
		"ca.pem": &bintree{signerCaPem, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
