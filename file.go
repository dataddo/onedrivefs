package onedrivefs

import (
	"io"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"time"
)

type openFile struct {
	fileInfo
	data io.ReadCloser
}

var _ fs.File = &openFile{}

func (f *openFile) Stat() (fs.FileInfo, error)     { return &f.fileInfo, nil }
func (f *openFile) Read(bytes []byte) (int, error) { return f.data.Read(bytes) }
func (f *openFile) Close() error                   { return f.data.Close() }

type openDir struct {
	fileInfo
	fs      *FS
	dirID   string
	driveID string

	getItemsOnce sync.Once
	items        []*driveItem
	offset       int
}

var (
	_ fs.File        = &openDir{}
	_ fs.ReadDirFile = &openDir{}
)

func (d *openDir) Stat() (fs.FileInfo, error) { return &d.fileInfo, nil }

func (d *openDir) ReadDir(count int) ([]fs.DirEntry, error) {
	var err error
	d.getItemsOnce.Do(func() {
		// We must get all the items, because the API does not support pagination.
		// It does support $top, but not $skip. WTF Microsoft?
		var items *driveItemsResponse
		items, err = listDriveItems(d.fs.ctx, d.fs.client, d.driveID, d.dirID)
		if err == nil {
			d.items = items.DriveItems
		}
	})
	if err != nil {
		return nil, err
	}
	n := len(d.items) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		item := d.items[d.offset+i]
		mode := fs.FileMode(0o555)
		if item.Folder != nil {
			mode |= fs.ModeDir
		}
		list[i] = &dirEntry{
			fileInfo: fileInfo{
				name:    item.Name,
				size:    item.Size,
				mode:    mode,
				modTime: time.Time(item.LastModifiedDateTime),
				isDir:   item.Folder != nil,
			},
		}
	}
	d.offset += n
	// Some extra sorting, as the Microsoft API can't be trusted
	slices.SortFunc(list, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return list, nil
}

func (d *openDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}

func (d *openDir) Close() error { return nil }

type dirEntry struct{ fileInfo }

func (d *dirEntry) Name() string               { return d.name }
func (d *dirEntry) IsDir() bool                { return d.isDir }
func (d *dirEntry) Type() fs.FileMode          { return d.mode.Type() }
func (d *dirEntry) Info() (fs.FileInfo, error) { return &d.fileInfo, nil }

type fileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (f *fileInfo) Name() string       { return f.name }
func (f *fileInfo) Size() int64        { return f.size }
func (f *fileInfo) Mode() fs.FileMode  { return f.mode }
func (f *fileInfo) ModTime() time.Time { return f.modTime }
func (f *fileInfo) IsDir() bool        { return f.isDir }
func (f *fileInfo) Sys() any           { return nil }
