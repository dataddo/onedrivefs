package onedrivefs

import (
	"context"
	"errors"

	"golang.org/x/oauth2"
)

func Example() {
	ctx := context.TODO()

	config := &oauth2.Config{
		// The client ID and secret are required to authenticate with the OneDrive API.
	}
	tok := &oauth2.Token{
		// The access token is not needed for this example.
	}
	client := oauth2.NewClient(ctx, config.TokenSource(ctx, tok))
	// Create a new OneDrive client.
	fs, _ := OpenFS(client, DriveOpts{DriveID: ""})
	f, err := fs.Context(ctx).Open("mydir/foo.json")
	if err != nil {
		var odErr *OneDriveAPIError
		if errors.As(err, &odErr) && odErr.Code == ActivityLimitReachedErrorCode {
			// Handle rate limit error.
			odErr.ResponseHeader.Get("Retry-After")
		}
	}
	defer func() { _ = f.Close() }()
}
