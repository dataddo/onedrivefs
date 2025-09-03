package onedrivefs

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

func TestFS_Open(t *testing.T) {
	floorModtime := time.Date(2024, time.January, 0, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		filePath string
		want     fileInfo
		wantErr  func(*testing.T, error)
	}{
		{
			name:     "root dir",
			filePath: ".",
			want: fileInfo{
				name:    "onedrivefs_test",
				size:    62,
				mode:    0o555 + fs.ModeDir,
				modTime: floorModtime,
				isDir:   true,
			},
		},
		{
			name:     "root file",
			filePath: "README.md",
			want: fileInfo{
				name:    "README.md",
				size:    33,
				mode:    0o555,
				modTime: floorModtime,
				isDir:   false,
			},
		},
		{
			name:     "1st subdir",
			filePath: "subdir1",
			want: fileInfo{
				name:    "subdir1",
				size:    29,
				mode:    0o555 + fs.ModeDir,
				modTime: floorModtime,
				isDir:   true,
			},
		},
		{
			name:     "2nd subdir",
			filePath: "subdir1/subdir2",
			want: fileInfo{
				name:    "subdir2",
				size:    29,
				mode:    0o555 + fs.ModeDir,
				modTime: floorModtime,
				isDir:   true,
			},
		},
		{
			name:     "subdir file",
			filePath: "subdir1/subdir2/foo.json",
			want: fileInfo{
				name:    "foo.json",
				size:    9,
				mode:    0o555,
				modTime: floorModtime,
				isDir:   false,
			},
		},
		{
			name:     "non-existent file",
			filePath: "non-existent-file.json",
			wantErr: func(t *testing.T, err error) {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal("expected os.ErrNotExist, got:", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := initFileSystem(t)
			got, err := fsys.Open(tt.filePath)
			if tt.wantErr != nil {
				tt.wantErr(t, err)
				return
			}
			noErr(t, err)
			t.Cleanup(func() {
				err := got.Close()
				noErr(t, err)
			})
			stat, err := got.Stat()
			noErr(t, err)
			requireFileInfoEqual(t, tt.want, stat)
			if !tt.want.isDir {
				_, err := io.ReadAll(got)
				noErr(t, err)
			}
		})
	}
}

func TestFS_ReadDir(t *testing.T) {
	tests := []struct {
		name    string
		dirPath string
		want    []fileInfo
		wantErr func(*testing.T, error)
	}{
		{
			name:    "root dir",
			dirPath: ".",
			want: []fileInfo{
				{name: "README.md"},
				{name: "subdir1"},
			},
		},
		{
			name:    "subdir",
			dirPath: "subdir1",
			want: []fileInfo{
				{name: "subdir2"},
			},
		},
		{
			name:    "subdir2",
			dirPath: "subdir1/subdir2",
			want: []fileInfo{
				{name: "foo.json"},
				{name: "foo-json"},
			},
		},
		{
			name:    "non existent dir",
			dirPath: "no-dir",
			wantErr: func(t *testing.T, err error) {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal("expected os.ErrNotExist, got:", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := initFileSystem(t)
			got, err := fs.ReadDir(fsys, tt.dirPath)
			if tt.wantErr != nil {
				tt.wantErr(t, err)
				return
			}
			noErr(t, err)
			for _, file := range tt.want {
				if !slices.ContainsFunc(got, func(f fs.DirEntry) bool { return f.Name() == file.Name() }) {
					t.Errorf("ListFile: missing file %q", file.Name())
				}
			}
		})
	}
}

func TestFS_TestFS(t *testing.T) {
	fsys := initFileSystem(t)
	err := fstest.TestFS(fsys,
		"README.md",
		"subdir1/subdir2/foo.json",
		"subdir1/subdir2/foo-json",
	)
	noErr(t, err)
}

func TestFS_Context(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	client := initClient(t)
	fileSystem, err := OpenFS(client, DriveOpts{DriveID: ""})
	noErr(t, err)
	fileSystem = fileSystem.Context(ctx)
	err = fs.WalkDir(fileSystem, ".", func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		t.Log(path)
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expected context deadline exceeded error, got:", err)
	}
}

func initClient(t *testing.T) *http.Client {
	ctx := t.Context()
	expire, err := time.Parse(time.RFC3339, os.Getenv("OAUTH_EXPIRES_AT"))
	noErr(t, err)

	config := &oauth2.Config{
		ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
		Endpoint:     microsoft.AzureADEndpoint(os.Getenv("OAUTH_TENANT_ID")),
	}
	tok := &oauth2.Token{
		TokenType:    "bearer",
		AccessToken:  os.Getenv("OAUTH_ACCESS_TOKEN"),
		RefreshToken: os.Getenv("OAUTH_REFRESH_TOKEN"),
		Expiry:       expire,
	}
	tokSrc := config.TokenSource(ctx, tok)
	return oauth2.NewClient(ctx, tokSrc)
}

func initFileSystem(t *testing.T) fs.FS {
	authClient := initClient(t)
	var fsys fs.FS
	fsys, err := OpenFS(authClient, DriveOpts{DriveID: ""})
	noErr(t, err)
	fsys, err = fs.Sub(fsys, os.Getenv("ONEDRIVE_TEST_SUBDIR"))
	noErr(t, err)
	return fsys
}

func requireFileInfoEqual(t *testing.T, want fileInfo, stat fs.FileInfo) {
	t.Helper()
	name := stat.Name()
	assertEqual(t, want.name, name, name)
	size := stat.Size()
	assertEqual(t, want.size, size, name)
	mode := stat.Mode()
	assertEqual(t, want.mode.String(), mode.String(), name)
	modTime := stat.ModTime()
	// We can't compare the exact time because OneDrive test-set can be rebuilt and
	// the file's modtime can change. We can only say the modtime is present and not
	// zero. In a case of zero time, it matches thanks to the equal operator.
	if modTime.Before(want.modTime) {
		t.Errorf("File %q: want modTime >= %v, got %v", name, want.modTime, modTime)
	}
	isDir := stat.IsDir()
	assertEqual(t, want.isDir, isDir, name)
}

func assertEqual[T any](t *testing.T, want, got T, filename string) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("File %q: want %v, got %v", filename, want, got)
	}
}

func noErr(t *testing.T, err error, args ...any) {
	if err != nil {
		t.Fatal(append([]any{err}, args...)...)
	}
}
