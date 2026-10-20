package pkg

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type MalformedError struct{ Reason string }

func (e *MalformedError) Error() string { return "malformed package: " + e.Reason }
func Malformed(format string, args ...any) error {
	return &MalformedError{fmt.Sprintf(format, args...)}
}
func IsMalformed(err error) bool { var e *MalformedError; return errors.As(err, &e) }

type UnsupportedError struct{ Type string }

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("unsupported manifest format %q; use a compatible Signetleaf version", e.Type)
}

type Reader struct {
	ZIP           *zip.ReadCloser
	Entries       map[string]*zip.File
	Names         []string
	Manifest      Manifest
	ManifestBytes []byte
}

func Open(name string) (*Reader, error) {
	z, err := zip.OpenReader(name)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			return nil, Malformed("invalid ZIP")
		}
		return nil, err
	}
	r := &Reader{ZIP: z, Entries: map[string]*zip.File{}, Names: []string{}}
	fail := func(err error) (*Reader, error) { z.Close(); return nil, err }
	seen := map[string]bool{}
	for _, f := range z.File {
		n := strings.TrimSuffix(f.Name, "/")
		if err := SafePath(n); err != nil {
			return fail(Malformed("%v", err))
		}
		if seen[n] {
			return fail(Malformed("duplicate ZIP entry %q", n))
		}
		seen[n] = true
		if !f.Mode().IsRegular() && !f.Mode().IsDir() {
			return fail(Malformed("ZIP entry %q is not a regular file or folder", n))
		}
		if strings.HasSuffix(f.Name, "/") != f.Mode().IsDir() {
			return fail(Malformed("inconsistent ZIP entry %q", n))
		}
		if !f.Mode().IsDir() {
			r.Entries[n] = f
			r.Names = append(r.Names, n)
		}
	}
	for _, entry := range z.File {
		n := strings.TrimSuffix(entry.Name, "/")
		for parent := filepath.ToSlash(filepath.Dir(n)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			if _, exists := r.Entries[parent]; exists {
				return fail(Malformed("file %q is also an entry parent", parent))
			}
		}
	}
	sort.Strings(r.Names)
	r.ManifestBytes, err = r.Read("manifest.json")
	if err != nil {
		return fail(err)
	}
	if err = json.Unmarshal(r.ManifestBytes, &r.Manifest); err != nil {
		return fail(Malformed("invalid manifest.json: %v", err))
	}
	if r.Manifest.Type != ManifestType {
		if r.Manifest.Type == "" {
			return fail(Malformed("manifest type is missing"))
		}
		return fail(&UnsupportedError{r.Manifest.Type})
	}
	if err = r.Manifest.Validate(); err != nil {
		return fail(Malformed("%v", err))
	}
	return r, nil
}
func (r *Reader) Close() error { return r.ZIP.Close() }
func (r *Reader) Read(name string) ([]byte, error) {
	f, ok := r.Entries[name]
	if !ok {
		return nil, Malformed("missing %s", name)
	}
	if f.UncompressedSize64 > 16<<20 {
		return nil, Malformed("metadata %q exceeds 16 MiB", name)
	}
	in, err := f.Open()
	if err != nil {
		return nil, Malformed("cannot open %q: %v", name, err)
	}
	defer in.Close()
	b, err := io.ReadAll(io.LimitReader(in, (16<<20)+1))
	if err != nil {
		return nil, Malformed("cannot read %q: %v", name, err)
	}
	if len(b) > 16<<20 {
		return nil, Malformed("metadata %q exceeds 16 MiB", name)
	}
	return b, nil
}

type Difference struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

func (r *Reader) Check() ([]Difference, int, error) {
	differences := []Difference{}
	matching := 0
	listed := map[string]bool{}
	for _, rec := range r.Manifest.Files {
		listed[rec.Path] = true
		f, exists := r.Entries[rec.Path]
		if !exists {
			differences = append(differences, Difference{rec.Path, "missing"})
			continue
		}
		in, err := f.Open()
		if err != nil {
			differences = append(differences, Difference{rec.Path, "changed"})
			continue
		}
		h := sha256.New()
		size, err := io.Copy(h, in)
		closeErr := in.Close()
		if err != nil || closeErr != nil || size != rec.Size || hex.EncodeToString(h.Sum(nil)) != rec.SHA256 {
			differences = append(differences, Difference{rec.Path, "changed"})
		} else {
			matching++
		}
	}
	for _, name := range r.Names {
		if !listed[name] && !Exempt(name) {
			differences = append(differences, Difference{name, "unexpected"})
		}
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].Path < differences[j].Path })
	return differences, matching, nil
}
func BuildManifest(dir string, m *Manifest) error {
	m.Files = []File{}
	err := filepath.WalkDir(dir, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, full)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if err = SafePath(name); err != nil {
			return err
		}
		if Exempt(name) {
			return nil
		}
		f, err := os.Open(full)
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		ce := f.Close()
		if err != nil {
			return err
		}
		if ce != nil {
			return ce
		}
		m.Files = append(m.Files, File{name, n, hex.EncodeToString(h.Sum(nil))})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	if err = m.Validate(); err != nil {
		return err
	}
	b, err := JSON(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600)
}
func exists(name string) (bool, error) {
	_, err := os.Lstat(name)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
func Write(dir, out, name string, at time.Time) (result string, err error) {
	return WriteStem(dir, out, name+"-"+at.UTC().Format("2006-01-02-1504"))
}

// WriteStem writes a package exclusively, reserving its paired reader path too.
func WriteStem(dir, out, stem string) (result string, err error) {
	var f *os.File
	for n := 1; ; n++ {
		suffix := ""
		if n > 1 {
			suffix = fmt.Sprintf("-%d", n)
		}
		result = filepath.Join(out, stem+suffix+".signetleaf.zip")
		used, e := exists(strings.TrimSuffix(result, ".zip"))
		if e != nil {
			return "", e
		}
		if used {
			continue
		}
		f, e = os.OpenFile(result, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if os.IsExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		break
	}
	defer func() {
		if e := f.Close(); err == nil {
			err = e
		}
		if err != nil {
			os.Remove(result)
		}
	}()
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(dir, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(dir, full)
		if e != nil {
			return e
		}
		name := filepath.ToSlash(rel)
		if e = SafePath(name); e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("payload %q is not regular", name)
		}
		hdr, e := zip.FileInfoHeader(info)
		if e != nil {
			return e
		}
		hdr.Name = name
		hdr.Method = zip.Deflate
		dst, e := zw.CreateHeader(hdr)
		if e != nil {
			return e
		}
		src, e := os.Open(full)
		if e != nil {
			return e
		}
		_, e = io.Copy(dst, src)
		ce := src.Close()
		if e != nil {
			return e
		}
		return ce
	})
	if err != nil {
		zw.Close()
		return result, err
	}
	err = zw.Close()
	return result, err
}
