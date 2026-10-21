package repo

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
)

// CheckSnapshot also runs without a destination, before restore creates staging files.
func CheckSnapshot(r *packagefmt.Reader, prefix, destination string) error {
	raw, err := r.Read(prefix + "/files.json")
	if err != nil {
		return err
	}
	var files []File
	if err = json.Unmarshal(raw, &files); err != nil || files == nil {
		return packagefmt.Malformed("invalid repository files.json")
	}
	records := map[string]File{}
	links := map[string]string{}
	for _, f := range files {
		if err = packagefmt.SafePath(f.Path); err != nil {
			return packagefmt.Malformed("%v", err)
		}
		if _, ok := records[f.Path]; ok {
			return packagefmt.Malformed("duplicate snapshot file %q", f.Path)
		}
		mode, e := strconv.ParseUint(f.Mode, 8, 32)
		if e != nil || mode > 0777 || len(f.SHA256) != 64 || f.Size < 0 {
			return packagefmt.Malformed("invalid snapshot record %q", f.Path)
		}
		if f.LinkTarget != "" {
			if e = packagefmt.SafeLink(f.Path, f.LinkTarget); e != nil {
				return packagefmt.Malformed("%v", e)
			}
			links[f.Path] = f.LinkTarget
		}
		records[f.Path] = f
	}
	for _, record := range files {
		name := record.Path
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, ok := records[parent]; ok {
				return packagefmt.Malformed("snapshot file %q is also a parent", parent)
			}
		}
	}
	for _, record := range files {
		if record.LinkTarget == "" {
			continue
		}
		if err = checkLinkChain(record.Path, record.LinkTarget, links); err != nil {
			return packagefmt.Malformed("%v", err)
		}
	}
	entry, ok := r.Entries[prefix+"/snapshot.tar.gz"]
	if !ok {
		return packagefmt.Malformed("missing snapshot.tar.gz")
	}
	in, err := entry.Open()
	if err != nil {
		return packagefmt.Malformed("cannot open snapshot: %v", err)
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return packagefmt.Malformed("invalid snapshot gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return packagefmt.Malformed("invalid snapshot tar: %v", e)
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		if e = packagefmt.SafePath(name); e != nil {
			return packagefmt.Malformed("%v", e)
		}
		if seen[name] {
			return packagefmt.Malformed("duplicate snapshot entry %q", name)
		}
		seen[name] = true
		if hdr.Typeflag == tar.TypeDir {
			if _, ok := records[name]; ok {
				return packagefmt.Malformed("snapshot mode differs for %q", name)
			}
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeSymlink {
			return packagefmt.Malformed("unsupported snapshot entry %q", name)
		}
		f, ok := records[name]
		if !ok {
			return packagefmt.Malformed("unexpected snapshot member %q", name)
		}
		mode, _ := strconv.ParseInt(f.Mode, 8, 64)
		if hdr.Mode != mode {
			return packagefmt.Malformed("snapshot mode differs for %q", name)
		}
		if hdr.Typeflag == tar.TypeSymlink {
			if f.LinkTarget == "" || hdr.Linkname != f.LinkTarget || hdr.Size != 0 || f.Size != int64(len(hdr.Linkname)) || f.SHA256 != packagefmt.Hash([]byte(hdr.Linkname)) {
				return packagefmt.Malformed("snapshot link differs for %q", name)
			}
			continue
		}
		if f.LinkTarget != "" || hdr.Size != f.Size {
			return packagefmt.Malformed("snapshot type or size differs for %q", name)
		}
		h := sha256.New()
		dst := io.Writer(h)
		var out *os.File
		if destination != "" {
			full := filepath.Join(destination, filepath.FromSlash(name))
			if e = os.MkdirAll(filepath.Dir(full), 0755); e != nil {
				return e
			}
			out, e = os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			dst = io.MultiWriter(h, out)
		}
		_, e = io.Copy(dst, tr)
		if out != nil {
			if ce := out.Close(); e == nil {
				e = ce
			}
		}
		if e != nil {
			if destination != "" {
				return e
			}
			return packagefmt.Malformed("cannot read snapshot member %q: %v", name, e)
		}
		if hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
			return packagefmt.Malformed("snapshot hash differs for %q", name)
		}
		if destination != "" {
			full := filepath.Join(destination, filepath.FromSlash(name))
			if e = os.Chmod(full, os.FileMode(mode)); e != nil {
				return e
			}
			if e = os.Chtimes(full, hdr.ModTime, hdr.ModTime); e != nil {
				return e
			}
		}
	}
	// Consume the gzip trailer and ZIP checksum; tar stops at its end markers.
	if _, err = io.Copy(io.Discard, gz); err != nil {
		return packagefmt.Malformed("invalid snapshot gzip: %v", err)
	}
	for _, record := range files {
		name := record.Path
		if !seen[name] {
			return packagefmt.Malformed("missing snapshot member %q", name)
		}
	}
	// Explicit directory entries must not sit below regular files or links either.
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, ok := records[parent]; ok {
				return packagefmt.Malformed("snapshot file %q is also a parent", parent)
			}
		}
	}
	if destination != "" {
		for _, f := range files {
			if f.LinkTarget == "" {
				continue
			}
			full := filepath.Join(destination, filepath.FromSlash(f.Path))
			if err = os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				return err
			}
			if err = os.Symlink(f.LinkTarget, full); err != nil {
				return err
			}
		}
	}
	return nil
}
func checkLinkChain(name, target string, links map[string]string) error {
	pending := strings.Split(path.Dir(name)+"/"+target, "/")
	stack := []string{}
	expanded := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(stack) == 0 {
				return fmt.Errorf("link %q points outside the repository", name)
			}
			stack = stack[:len(stack)-1]
			continue
		}
		stack = append(stack, part)
		if next, ok := links[strings.Join(stack, "/")]; ok {
			expanded++
			if expanded > 40 {
				return fmt.Errorf("link cycle at %q", name)
			}
			stack = stack[:len(stack)-1]
			pending = append(strings.Split(next, "/"), pending...)
		}
	}
	return nil
}
