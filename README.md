# chief-go

Go client library for the [Chief](https://chief.bot/) Chief public REST API.

It depends only on the standard library, so it can be vendored into external tools without pulling in any third-party packages.

## Install

```bash
go get github.com/Storytell-ai/chief-go/chief
```

## Requirements

A Personal Access Token is required to interact with the Chief API. It is sent as the `X-API-Key` header.

Most routes are project-scoped and also need a project ID, sent as the `X-Project-Id` header. The `Projects.List` call is the exception: it works with only an API key and returns the projects the key can reach.

Credentials can be passed as options or read from the environment:

| Option | Environment variable | Required |
|--------|----------------------|----------|
| `WithAPIKey` | `CHIEF_API_KEY` | yes |
| `WithProjectID` | `CHIEF_PROJECT_ID` | for project-scoped routes |
| `WithBaseURL` | `CHIEF_BASE_URL` | no (defaults to `https://api.storytell.ai`) |

## API

Resources supported by this package:

- [x] Assets (`/v1/assets`)
- [x] Labels (`/v1/labels`)
- [x] Actions (`/v1/actions`)
- [x] Live-Sessions (`/v1/sessions`)
- [x] Skills (`/v1/skills`)
- [x] Memories (`/v1/memories`)
- [x] Projects (`/v1/projects`)

## Usage

Here is an example that uploads a local file and labels it. `UploadFile` runs the full three-step flow: create the asset row, `PUT` the bytes to the signed URL, then complete the upload to start ingest.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Storytell-ai/chief-go/chief"
)

func main() {
	// credentials are read from CHIEF_API_KEY and CHIEF_PROJECT_ID by default;
	// the options below make that explicit.
	client, err := chief.New(
		chief.WithAPIKey(os.Getenv("CHIEF_API_KEY")),
		chief.WithProjectID(os.Getenv("CHIEF_PROJECT_ID")),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// upload a local file; the bool is true when the content was a dedup hit
	// and no bytes were uploaded.
	asset, deduped, err := client.Assets.UploadFile(ctx, "report.pdf")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("asset %q (dedup hit: %t)\n", asset.AssetID, deduped)

	// block until ingest reaches a terminal status.
	asset, err = client.Assets.WaitForReady(ctx, asset.AssetID, 2*time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status: %s\n", asset.Status)

	// attach a label by name; a name with no matching label is auto-created.
	label, err := client.Assets.AttachLabel(ctx, asset.AssetID, "quarterly")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("attached label %q (%s)\n", label.Name, label.LabelID)

	// list the assets in the project.
	page, err := client.Assets.List(ctx, chief.WithLimit(10))
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range page.Data {
		fmt.Printf("%s\t%s\t%s\n", a.AssetID, a.Status, a.Filename)
	}
}
```

Each resource is its own service on the client. Here is an example that creates a scheduled action that emails its result:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Storytell-ai/chief-go/chief"
)

func main() {
	client, err := chief.New(
		chief.WithAPIKey(os.Getenv("CHIEF_API_KEY")),
		chief.WithProjectID(os.Getenv("CHIEF_PROJECT_ID")),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// create an action that runs every day at 09:00 and emails its result.
	action, err := client.Actions.Create(ctx, &chief.ActionRequest{
		Name:   "daily-digest",
		Prompt: "Summarize everything uploaded in the last 24 hours.",
		Schedule: &chief.ScheduleRequest{
			Hour:     "9",
			Weekday:  "*",
			MonthDay: "*",
			Timezone: "America/Sao_Paulo",
		},
		Email: &chief.EmailOutcome{
			To:                   []string{"team@example.com"},
			Subject:              "Daily digest",
			IncludeDateInSubject: true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("created action %q\n", action.ActionID)

	// pause it, then resume it.
	_, _ = client.Actions.Disable(ctx, action.ActionID)
	_, _ = client.Actions.Enable(ctx, action.ActionID)
}
```

## Errors

Every non-2xx response is returned as an `*APIError` carrying the HTTP status, a stable machine-readable `Code`, and a user-facing `Humane` message. Helpers cover the common cases:

```go
session, err := client.Sessions.Get(ctx, "missing-id")
if err != nil {
	switch {
	case chief.IsNotFound(err):
		// 404
	default:
		if apiErr, ok := chief.IsAPIError(err); ok {
			log.Printf("chief: %s (%s)", apiErr.Humane, apiErr.Code)
		}
	}
}
```

## Development

This project uses [Task](https://taskfile.dev/) for common workflows:

```bash
task build    # compile all packages
task test     # run the test suite
task lint     # run golangci-lint
task fmt      # format the code
task release  # cut a release with goreleaser
```

Run `task` with no arguments to list every available task.
