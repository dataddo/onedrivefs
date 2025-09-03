package onedrivefs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

// getDriveItemsByPath is an extension to
// (*onedrive.DriveItemsService).GetByPath allowing to get items from a specific
// drive of the authenticated user.
//
// OneDrive API docs: https://docs.microsoft.com/en-us/onedrive/developer/rest-api/api/driveitem_get
func getDriveItemsByPath(ctx context.Context, client *http.Client, driveID, itemPath string) (*driveItem, error) {
	apiURL := "me/drive/root"
	if driveID != "" {
		apiURL = "/v1.0/drives/" + url.PathEscape(driveID) + "/root"
	}
	if itemPath != "" {
		apiURL += ":/" + url.PathEscape(itemPath)
	}
	req, err := newRequest("GET", apiURL)
	if err != nil {
		return nil, err
	}
	var driveItem *driveItem
	if err := doRequest(ctx, client, req, &driveItem); err != nil {
		return nil, err
	}
	return driveItem, nil
}

// driveItem represents a OneDrive drive item.
// Ref https://docs.microsoft.com/en-us/graph/api/resources/driveitem?view=graph-rest-1.0
// It's an extended version of onedrive.DriveItem.
type driveItem struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	DownloadURL          string         `json:"@microsoft.graph.downloadUrl"`
	Description          string         `json:"description"`
	Folder               *struct{}      `json:"folder"`
	Root                 *struct{}      `json:"root"`
	Size                 int64          `json:"size"`
	CreatedDateTime      dateTimeOffset `json:"createdDateTime"`
	LastModifiedDateTime dateTimeOffset `json:"lastModifiedDateTime"`
}

type dateTimeOffset time.Time

func (d *dateTimeOffset) UnmarshalText(text []byte) error {
	t, err := time.Parse(time.RFC3339, string(text))
	if err != nil {
		return err
	}
	*d = dateTimeOffset(t)
	return nil
}

// listDriveItems lists the items of a folder of the authenticated user. It's an
// extension to (*onedrive.DriveItemsService).List method.
//
// OneDrive API docs: https://docs.microsoft.com/en-us/onedrive/developer/rest-api/resources/driveitem?view=odsp-graph-online
func listDriveItems(ctx context.Context, client *http.Client, driveID, folderID string) (*driveItemsResponse, error) {
	apiURL := "me/drive/root/children"
	if folderID != "" {
		apiURL = "me/drive/items/" + url.PathEscape(folderID) + "/children"
	}
	if driveID != "" {
		apiURL = "me/drives/" + url.PathEscape(driveID) + "/root/children"
		if folderID != "" {
			apiURL = "me/drives/" + url.PathEscape(driveID) + "/items/" + url.PathEscape(folderID) + "/children"
		}
	}
	req, err := newRequest("GET", apiURL)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = url.Values{
		"$orderby": {"name asc"},
	}.Encode()
	var oneDriveResponse *driveItemsResponse
	if err := doRequest(ctx, client, req, &oneDriveResponse); err != nil {
		return nil, err
	}
	return oneDriveResponse, nil
}

// driveItemsResponse represents the JSON object returned by the OneDrive API.
// It's an extended version of onedrive.OneDriveDriveItemsResponse.
type driveItemsResponse struct {
	ODataContext string       `json:"@odata.context"`
	Count        int          `json:"@odata.count"`
	DriveItems   []*driveItem `json:"value"`
}

var baseURL = url.URL{
	Scheme: "https",
	Host:   "graph.microsoft.com",
	Path:   "/v1.0/",
}

func newRequest(method, relativeURL string) (*http.Request, error) {
	apiURL, err := baseURL.Parse(relativeURL)
	if err != nil {
		return nil, err
	}
	return http.NewRequest(method, apiURL.String(), nil)
}

func doRequest(ctx context.Context, client *http.Client, req *http.Request, target interface{}) error {
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		var oneDriveError struct {
			Error *OneDriveAPIError `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&oneDriveError); err != nil {
			return errors.New("unexpected error: " + resp.Status)
		}
		if oneDriveError.Error != nil {
			oneDriveError.Error.ResponseHeader = resp.Header
			return oneDriveError.Error
		}
		return errors.New("unexpected error: " + resp.Status)
	}
	if resp.StatusCode != 204 {
		err = json.NewDecoder(resp.Body).Decode(target)
	}
	return err
}
