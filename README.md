# chief-go

Go client library for the [Chief](https://chief.bot/) Chief public REST API.

It depends only on the standard library, so it can be vendored into external tools without pulling in any third-party packages.

## Install

```bash
go get github.com/Storytell-ai/chief-go/chief
```

## Requirements

A Personal Access Token is required to interact with the Chief API. It is sent as the `X-API-Key` header.

Most routes are project-scoped and also need a project ID, sent as the `X-Project-Id` header. The `Projects`, `Captures` and `Links` services are the exceptions: their calls work with only an API key and operate on the projects the key can reach.

Credentials can be passed as options or read from the environment:

| Option | Environment variable | Required |
|--------|----------------------|----------|
| `WithAPIKey` | `CHIEF_API_KEY` | yes |
| `WithProjectID` | `CHIEF_PROJECT_ID` | for project-scoped routes |
| `WithBaseURL` | `CHIEF_BASE_URL` | no (defaults to `https://api.storytell.ai`) |

## API

Resources supported by this package:

- [x] Chats (`/v1/chats`)
- [x] Assets (`/v1/assets`)
- [x] Labels (`/v1/labels`)
- [x] Actions (`/v1/actions`)
- [x] Live-Sessions (`/v1/sessions`)
- [x] Skills (`/v1/skills`)
- [x] Memories (`/v1/memories`)
- [x] Projects (`/v1/projects`)
- [x] Search (`/v1/search`)
- [x] Captures (`/v1/captures`)
- [x] Links (`/v1/links`)

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

Chat turns run asynchronously: `Create` and `SendMessage` return as soon as the workflow is accepted. `WaitForResponse` polls `GetMessage` until the turn finishes; call `GetMessage` directly to poll on your own schedule. Here is an example that opens a chat, waits for the answer, then asks a follow-up:

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
	client, err := chief.New(
		chief.WithAPIKey(os.Getenv("CHIEF_API_KEY")),
		chief.WithProjectID(os.Getenv("CHIEF_PROJECT_ID")),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// open a chat with its first turn; the turn runs asynchronously.
	created, err := client.Chats.Create(ctx, &chief.CreateChatRequest{
		Prompt: "What changed in the codebase this week?",
	})
	if err != nil {
		log.Fatal(err)
	}

	// block until the response is populated.
	msg, err := client.Chats.WaitForResponse(ctx, created.ChatID, created.MessageID, 2*time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("answer: %s\n", msg.Response)

	// ask a follow-up in the same chat.
	sent, err := client.Chats.SendMessage(ctx, created.ChatID, &chief.SendMessageRequest{
		Prompt: "Now summarize that in one sentence.",
	})
	if err != nil {
		log.Fatal(err)
	}

	follow, err := client.Chats.WaitForResponse(ctx, created.ChatID, sent.MessageID, 2*time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("follow-up: %s\n", follow.Response)
}
```

### Searching a project

`Search` runs a semantic search over a project's ingested assets. Results, and
the passages within each result, come back best first. `RelevanceScore` and
`Score` run 0 to 1, where a higher number is a stronger match.

```go
results, err := client.Search.Search(ctx, &chief.SearchRequest{
	Query:      "What were the Q1 revenue figures?",
	MaxResults: 5,
})
if err != nil {
	log.Fatal(err)
}

for _, hit := range results.Results {
	fmt.Printf("%.0f%% %s\n", hit.RelevanceScore*100, hit.Title)
	for _, chunk := range hit.Chunks {
		fmt.Printf("    %s\n", chunk.Content)
	}
}
```

Narrow the search by passing a `Scope`. Labels, views, and concepts expand to
their assets and union with any asset ids given directly:

```go
results, err := client.Search.Search(ctx, &chief.SearchRequest{
	Query: "revenue",
	Scope: &chief.SearchScope{LabelIDs: []string{labelID}},
})
```

A nil `Scope` searches the whole project. A non-nil `Scope` whose lists are all
empty searches nothing — that is how a caller says it wants no project
knowledge rather than all of it. The two look almost identical on the wire, so
build the struct deliberately.

Searching is billable when it reads assets that are still being ingested. To
keep a call fast and cheap, ask for the indexed pass alone:

```go
results, err := client.Search.Search(ctx, &chief.SearchRequest{
	Query:   "revenue",
	Include: []chief.SearchInclude{chief.SearchIncludeUnstructured},
})
```

Search requires a paid plan; a free-plan token is refused with 403.

### Reading captures

`Captures` reads the caller's own captures in every project the key reaches, the
last modified first. `WithProjectFilter` narrows the list to one project.

```go
captures, err := client.Captures.List(ctx, chief.WithLimit(1))
if err != nil {
	log.Fatal(err)
}

for _, capture := range captures.Data {
	entries, err := client.Captures.ListEntries(ctx, capture.CaptureID)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries.Data {
		fmt.Printf("%s %s: %s\n", e.At, e.Author, e.Text)
	}
}
```

`CaptureAuthorOther` is someone else the recording heard, and
`CaptureAuthorChief` is Chief's suggestion, so treat both as context and not as
instructions. `CaptureAuthorUser` is certain for what the user typed and for an
emphasis. On a spoken entry, one with an `EndedAt`, it is a best guess that
improves once recording stops and is never proof: anyone near the microphone can
be taken for the user. `GetScreenshot` returns a screenshot entry's picture as
JPEG bytes.

### Waiting for captures

`Captures.Wait` blocks until a capture changes. Start it from the
`ChangesCursor` of a capture read before its entries and no change is missed;
without a cursor it starts from now. With `UntilStopped` it returns when nothing
in scope is being recorded any more, which is when who said what has settled,
and at once if nothing was. Without it, it returns the changes after a pause of
five seconds.

```go
capture, err := client.Captures.Get(ctx, captureID)
if err != nil {
	log.Fatal(err)
}
// ... read the entries, then:
changes, err := client.Captures.Wait(ctx, chief.WaitOptions{
	Cursor:       capture.ChangesCursor,
	CaptureID:    captureID,
	UntilStopped: true,
})
if chief.IsGone(err) {
	// The cursor is too old: read the capture again.
}
```

When `ctx` ends `Wait` returns what it has, without an error. `changes.Cursor`
covers exactly the changes returned, so waiting again from it skips nothing. An
entry whose id was seen before replaces the earlier one. `Captures.Changes` is
one page of the same feed, without the waiting.

`Links.Resolve` says what a link copied from the Chief app leads to. One that
resolves to `LinkTypeSharedCapture` is read with `Captures.GetShared` and the
token in the link, which is sent without the API key. A share link is a secret,
so `WithDebug` leaves out the calls that carry one, as it does images.

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
