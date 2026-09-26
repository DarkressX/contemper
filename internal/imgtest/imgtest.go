// Package imgtest builds synthetic in-process OCI images for tests, so
// merge/validate/support logic can be exercised without a registry or a
// real container build.
package imgtest

import (
	"archive/tar"
	"bytes"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// File describes one tar entry to place in a synthetic layer.
type File struct {
	Path               string
	Typeflag           byte // defaults to tar.TypeReg
	Mode               int64
	Uid, Gid           int
	Data               []byte
	Linkname           string
	Devmajor, Devminor int64
}

var epoch = time.Unix(0, 0)

// Layer builds a v1.Layer containing files, in order.
func Layer(files []File) (v1.Layer, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range files {
		typeflag := f.Typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		mode := f.Mode
		if mode == 0 {
			switch typeflag {
			case tar.TypeDir:
				mode = 0o755
			default:
				mode = 0o644
			}
		}
		hdr := &tar.Header{
			Name:     f.Path,
			Typeflag: typeflag,
			Mode:     mode,
			Uid:      f.Uid,
			Gid:      f.Gid,
			Linkname: f.Linkname,
			Devmajor: f.Devmajor,
			Devminor: f.Devminor,
			Size:     int64(len(f.Data)),
			ModTime:  epoch,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if len(f.Data) > 0 {
			if _, err := tw.Write(f.Data); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return tarball.LayerFromReader(bytes.NewReader(buf.Bytes()))
}

// Image builds a v1.Image for platform with the given labels, from one
// layer per []File in layers (applied bottom to top).
func Image(platform v1.Platform, labels map[string]string, layers ...[]File) (v1.Image, error) {
	img := empty.Image
	for _, lf := range layers {
		l, err := Layer(lf)
		if err != nil {
			return nil, err
		}
		img, err = mutate.AppendLayers(img, l)
		if err != nil {
			return nil, err
		}
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	cfg = cfg.DeepCopy()
	cfg.OS = platform.OS
	cfg.Architecture = platform.Architecture
	if cfg.Config.Labels == nil {
		cfg.Config.Labels = map[string]string{}
	}
	for k, v := range labels {
		cfg.Config.Labels[k] = v
	}
	return mutate.ConfigFile(img, cfg)
}

// WhiteoutFile returns a File that whites out name in a higher layer.
func WhiteoutFile(name string) File {
	dir, base := splitPath(name)
	p := base
	if dir != "" {
		p = dir + "/.wh." + base
	} else {
		p = ".wh." + base
	}
	return File{Path: p, Typeflag: tar.TypeReg}
}

func splitPath(p string) (dir, base string) {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i], p[i+1:]
		}
	}
	return "", p
}
