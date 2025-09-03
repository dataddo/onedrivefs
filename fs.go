package onedrivefs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type FS struct {
	client *http.Client
	opts   DriveOpts
	ctx    context.Context
}

type DriveOpts struct {
	DriveID string
}

func OpenFS(client *http.Client, opts DriveOpts) (*FS, error) {
	return &FS{
		ctx:    context.Background(),
		client: client,
		opts:   opts,
	}, nil
}

var (
	_ fs.FS         = &FS{}
	_ fs.ReadDirFS  = &FS{}
	_ fs.ReadFileFS = &FS{}
	_ fs.StatFS     = &FS{}
	// _ fs.SubFS      = &FS{} // shifts the root directory, won't do now
	// _ fs.GlobFS     = &FS{} // not implemented
)

func (f *FS) Context(ctx context.Context) *FS {
	if ctx == nil {
		ctx = context.Background()
	}
	return &FS{
		ctx:    ctx,
		client: f.client,
		opts:   f.opts,
	}
}

func (f *FS) Open(origName string) (fs.File, error) {
	name := origName
	if err := validatePath(name); err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	// no working directory, start from the root
	if name == "." {
		name = "/"
	}
	itemPath := strings.TrimPrefix(name, "/")
	item, err := getDriveItemsByPath(f.ctx, f.client, f.opts.DriveID, itemPath)
	if err != nil {
		if odErr := (&OneDriveAPIError{}); errors.As(err, &odErr) && odErr.Code == ItemNotFoundErrorCode {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	if item.Folder != nil {
		name := item.Name
		if item.Root != nil {
			name = "."
		}
		return &openDir{
			fs:      f,
			driveID: f.opts.DriveID,
			dirID:   item.ID,
			fileInfo: fileInfo{
				isDir:   true,
				name:    name,
				mode:    0o555 + fs.ModeDir,
				modTime: time.Time(item.LastModifiedDateTime),
				size:    item.Size,
			},
		}, nil
	}
	if item.DownloadURL == "" {
		return nil, fmt.Errorf("the file is not downloadable, because the API didn't provide download URL")
	}
	downloadReq, err := http.NewRequestWithContext(f.ctx, "GET", item.DownloadURL, nil)
	if err != nil {
		return nil, err
	}
	// create new client to avoid using default client
	resp, err := (&http.Client{}).Do(downloadReq)
	if err != nil {
		return nil, err
	}

	return &openFile{
		fileInfo: fileInfo{
			isDir:   false,
			name:    item.Name,
			size:    item.Size,
			mode:    0o555,
			modTime: time.Time(item.LastModifiedDateTime),
		},
		data: resp.Body,
	}, nil
}

func (f *FS) ReadFile(name string) ([]byte, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var buf bytes.Buffer
	if info, err := file.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			buf.Grow(int(info.Size()))
		}
	}
	if _, err := io.Copy(&buf, file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	dir, ok := file.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not implemented")}
	}
	return dir.ReadDir(-1)
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	if err := validatePath(name); err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if name == "." {
		name = "/"
	}
	itemPath := strings.TrimPrefix(name, "/")
	item, err := getDriveItemsByPath(f.ctx, f.client, f.opts.DriveID, itemPath)
	if err != nil {
		if odErr := (&OneDriveAPIError{}); errors.As(err, &odErr) && odErr.Code == "itemNotFound" {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	if item.Folder != nil {
		name := item.Name
		if item.Root != nil {
			name = "."
		}
		return &fileInfo{
			isDir:   true,
			name:    name,
			mode:    0o555 + fs.ModeDir,
			modTime: time.Time(item.LastModifiedDateTime),
			size:    item.Size,
		}, nil
	}
	return &fileInfo{
		isDir:   false,
		name:    item.Name,
		size:    item.Size,
		mode:    0o555,
		modTime: time.Time(item.LastModifiedDateTime),
	}, nil
}

func validatePath(path string) error {
	if filepath.IsAbs(path) {
		return errors.New("absolute paths are not allowed")
	}
	if filepath.Clean(path) != path {
		return errors.New("relative path elements are not allowed")
	}
	// This is needed to pass `fstest.TestFS`
	if strings.Contains(path, `\`) {
		return errors.New("backslashes are not allowed in path")
	}
	return nil
}
