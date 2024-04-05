// gar
package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type tarOp func(string, *tar.Reader, *tar.Header) error

type config struct {
	tarPath  string
	dstPath  string
	srcPaths []string
	option   int
}

const (
	archive = iota
	unarchive
	list
)

const usage = `
gar(bsdtar): manipulate archive files
First option must be a mode specifier:
  -c Create  -r Add/Replace  -t List  -u Update  -x Extract
Common Options:
  -b #  Use # 512-byte records per I/O block
  -f <filename>  Location of archive
  -v    Verbose
  -w    Interactive
Create: tar -c [options] [<file> | <dir> | @<archive> | -C <dir> ]
  <file>, <dir>  add these items to archive
  -z, -j, -J, --lzma  Compress archive with gzip/bzip2/xz/lzma
  --format {ustar|pax|cpio|shar}  Select archive format
  --exclude <pattern>  Skip files that match pattern
  -C <dir>  Change to <dir> before processing remaining files
  @<archive>  Add entries from <archive> to output
List: tar -t [options] [<patterns>]
  <patterns>  If specified, list only entries that match
Extract: tar -x [options] [<patterns>]
  <patterns>  If specified, extract only entries that match
  -k    Keep (don't overwrite) existing files
  -m    Don't restore modification times
  -O    Write entries to stdout, don't restore to disk
  -p    Restore permissions (including ACLs, owner, file flags)
`

// TODO: add destination -> defaults to "."
func extractTarFile(_ string, r *tar.Reader, h *tar.Header) error {
	// targetPath := filepath.Join(dstPath, h.Name)
	targetPath := ""

	switch h.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(targetPath, os.FileMode(h.Mode)); err != nil {
			return fmt.Errorf("could not create dir in %s with %w", targetPath, err)
		}
		if err := os.Chmod(targetPath, os.FileMode(h.Mode)); err != nil {
			return fmt.Errorf("could not set permissions for dir in %s with %w", targetPath, err)
		}

	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("couldn't open dir in %s with %w", targetPath, err)
		}
		f, err := os.OpenFile(filepath.Join(targetPath, h.Name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode))
		if err != nil {
			return fmt.Errorf("couldn't open file in %s with %w", targetPath, err)
		}
		n, err := io.CopyN(f, r, h.Size)
		if err != nil {
			return fmt.Errorf("failed writing to file: %v, %d bytes, only wrote %d with %w", f, h.Size, n, err)
		}
		f.Close()
	}

	return nil
}

func createTarFile(tarPath string, srcPath ...string) error {
	dst, err := os.Create(tarPath)
	if err != nil {
		panic(err)
	}
	defer dst.Close()

	w := tar.NewWriter(dst)
	defer w.Close()

	for _, src := range srcPath {
		if err := writeToTar(w, src, ""); err != nil {
			return err
		}
	}

	return nil
}

func writeToTar(w *tar.Writer, srcPath, baseDir string) error {
	fi, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	h, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	if baseDir != "" {
		h.Name = filepath.Join(baseDir, filepath.Base(srcPath))
	}
	if fi.IsDir() {
		if err := w.WriteHeader(h); err != nil {
			return err
		}
		// Read directory contents
		entries, err := os.ReadDir(srcPath)
		if err != nil {
			return err
		}
		// Recursively add directory contents to tar
		for _, entry := range entries {
			entryPath := filepath.Join(srcPath, entry.Name())
			if err := writeToTar(w, entryPath, h.Name); err != nil {
				return err
			}
		}

		return nil
	}
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	if err := w.WriteHeader(h); err != nil {
		file.Close() // Make sure to close the file on error
		return err
	}
	_, err = io.Copy(w, file)
	file.Close() // Close after copying
	if err != nil {
		return err
	}
	return nil
}

func listTarFiles(_ string, _ *tar.Reader, h *tar.Header) error {
	fmt.Println(h.Name)
	return nil
}

func setCompressorType(file *os.File, compressorType string) (io.Reader, error) {
	switch compressorType {
	case "gzip":
		return gzip.NewReader(file)
	default:
		return file, nil
	}
}

func iterateTarEntries(tarPath, compressorType string, op tarOp) error {
	file, err := os.Open(tarPath)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("failed opening tarPath: %w", err)
	}

	reader, err := setCompressorType(file, compressorType)
	if err != nil {
		return fmt.Errorf("failed opening compressor reader of type %s with: %w", compressorType, err)
	}

	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed reading tar with: %w", err)
		}
		if err := op(tarPath, tarReader, header); err != nil {
			return fmt.Errorf("failed running operation with: %w", err)
		}
	}
	return nil
}

func main() {
	// TODO: turn into config struct
	listFlag := flag.String("t", "", "List file names in archive")
	createFlag := flag.Bool("c", false, "Create tar from path[s]")
	extractFlag := flag.String("x", "", "Extract tar to specific path")
	// TODO: add support for different encoding types. "gzip" already supported
	// compressFlag := flag.String("u", "", "Compress type (gzip, bz2, etc.)")

	flag.Parse()

	args := flag.Args()

	fmt.Println(*listFlag, *createFlag, *extractFlag, args)

	var err error
	switch {
	case *listFlag != "":
		err = iterateTarEntries(*listFlag, "", listTarFiles)
	case *createFlag:
		err = createTarFile(args[0], args[1:]...)
	case *extractFlag != "":
		err = iterateTarEntries(*extractFlag, "", extractTarFile)
	default:
		fmt.Printf(usage)
		os.Exit(1)
	}

	if err != nil {
		panic(err)
	}
}
