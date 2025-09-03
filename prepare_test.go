package onedrivefs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

func Test_createEnv(t *testing.T) {
	if os.Getenv("PREPARE_ONEDRIVEFS_TEST") != "YES" {
		t.Skip("PREPARE_ONEDRIVEFS_TEST not set, skipping OneDrive test data preparation")
	}

	authClient := initClient(t)

	var newDirID string
	switch testSubdir := os.Getenv("ONEDRIVE_TEST_SUBDIR"); testSubdir {
	case "":
		t.Fatal("ONEDRIVE_TEST_SUBDIR not set, set it to '.' or a subdirectory name")
	case ".":
		newDirID = "root"
	default:
		newDirID = mkdir(t, authClient, "root", testSubdir)
	}

	newSubDir1ID := mkdir(t, authClient, newDirID, "subdir1")
	newSubDir2ID := mkdir(t, authClient, newSubDir1ID, "subdir2")

	uploadFile(t, authClient, newDirID, "README.md", "text/plain", strings.NewReader(`This is a test dir for onedrivefs`))
	uploadFile(t, authClient, newSubDir2ID, "foo.json", "application/json", strings.NewReader(`"is JSON"`))
	uploadFile(t, authClient, newSubDir2ID, "foo.csv", "text/csv", strings.NewReader("foo,bar\n1,2\n"))
	uploadFile(t, authClient, newSubDir2ID, "foo-json", "text/plain", strings.NewReader(`not JSON`))
}

func mkdir(t *testing.T, authClient *http.Client, parentID string, folderName string) (itemID string) {
	reqBody, err := json.Marshal(struct {
		Name             string   `json:"name"`
		Folder           struct{} `json:"folder"`
		ConflictBehavior string   `json:"@microsoft.graph.conflictBehavior"`
	}{
		Name:             folderName,
		ConflictBehavior: "replace",
	})
	noErr(t, err)
	apiURL := "https://graph.microsoft.com/v1.0/me/drive/items/" + url.PathEscape(parentID) + "/children"
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(reqBody))
	noErr(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := authClient.Do(req.WithContext(t.Context()))
	noErr(t, err)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, err := io.ReadAll(resp.Body)
		noErr(t, err)
		t.Fatal("ERROR:", body)
	}
	var respPayload struct {
		Id string `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	noErr(t, err)
	if respPayload.Id == "" {
		noErr(t, fmt.Errorf("failed to create folder, empty ID in response"))
	}
	return respPayload.Id
}

func uploadFile(t *testing.T, authClient *http.Client, dirID, fileName, fileType string, fileData io.Reader) {
	req, err := http.NewRequest(
		"PUT",
		"https://graph.microsoft.com/v1.0/me/drive/items/"+url.PathEscape(dirID)+":/"+url.PathEscape(fileName)+":/content?@microsoft.graph.conflictBehavior=replace",
		fileData,
	)
	noErr(t, err)
	req.Header.Set("Content-Type", fileType)
	resp, err := authClient.Do(req.WithContext(t.Context()))
	noErr(t, err)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, err := io.ReadAll(resp.Body)
		noErr(t, err)
		t.Fatal("ERROR:", body)
	}
}
