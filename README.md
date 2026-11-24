# Content Automation Engine V2 (from Tiktok Incubator Project)

Scrapes stories from Reddit into MongoDB and, when the publishing pipeline is switched on,
turns them into narrated vertical videos and posts them.

## Architecture
- Event Driven Modular Monolith
- Native Go concurrency and event buses
- Written In Golang
- Events notify that something has happened; consumers query the producer's API for details
- Every event channel has exactly one producer and one consumer
- Inter-service transport (e.g. gRPC) is TBD; module contracts stay Go interfaces until then
- Each microservice owns it's contracts for other services to communicate with it
- One deployment process with all services in the backend

```
Reddit ──▶ Scraper ──posts──▶ Content ──▶ stories (MongoDB)

Scheduler ──TopicTriggered──▶ Content ──StorySelected──▶ Generator ──VideoReady──▶ Poster ──▶ TikTok
                                 ▲                          │ GetStoryByID            │ GetVideoByID
                                 └── PostedStoryIDs ◀───────┴─────────────────────────┘
                                                                     posted_stories (MongoDB)
```

| Module | Path | What it does |
|---|---|---|
| Content | `incubator/internal/content` | Stores scraped stories; claims one for the pipeline when the scheduler fires |
| Scheduler | `incubator/internal/scheduler` | Emits `TopicTriggered` on an interval |
| Generator | `incubator/internal/generator` | Narrates a story (Kokoro text-to-speech) and renders a 1080x1920 captioned video (ffmpeg) |
| Poster | `incubator/internal/poster` | Publishes each video once and records it in its own `posted_stories` collection |

Each module's contracts live in its `api` package. The wiring is in `incubator/internal/app`.

### How a story is never posted twice
1. **Claiming.** The Content service picks a story and marks it as claimed in one atomic MongoDB
   operation, so two callers (or two instances) can never be handed the same story. A claim
   expires after `CONTENT_CLAIM_TTL`, so a story whose video or upload failed is retried later,
   up to `CONTENT_MAX_ATTEMPTS` times.
2. **Recording.** The Poster service owns the `posted_stories` collection, which has a unique
   index on the story ID. It records the story *before* uploading; if the record already exists
   the upload is skipped. The Content service asks the Poster which stories are recorded and
   never selects those again.

## Running it

By default the engine **only scrapes**: it needs MongoDB and Reddit credentials and nothing else.
Set `PIPELINE_ENABLED=true` to also schedule, generate and post.

Run from `incubator/`, with the variables below set in your shell (`go run` does not read `.env`):

```bash
go run ./cmd/engine
```

The process exits with status 1 and logs the reason if it cannot start. `GET /healthz` on
`HTTP_PORT` returns 200 when every service is healthy and 503 otherwise.

### Docker
- `docker build .` builds the full image (ffmpeg, Chromium, fonts) that can run the whole pipeline.
- `docker build --target minimal .` builds a tiny image that can only scrape.
- `docker compose --profile pipeline up` also starts a local Kokoro text-to-speech server.

## Configuration

Always needed:

| Variable | Default | Meaning |
|---|---|---|
| `MONGO_USERNAME`, `MONGO_PASSWORD`, `MONGO_HOST` | required | MongoDB connection |
| `MONGO_APP_NAME` | empty | `appName` sent to MongoDB |
| `MONGO_DATABASE` | `prod` | Database holding every collection |
| `ENV` | empty | `dev` connects with `mongodb://`; anything else uses `mongodb+srv://` |
| `SCRAPER_APP_ID`, `SCRAPER_APP_SECRET` | required | The Reddit app's client ID and secret |
| `SCRAPER_USERNAME`, `SCRAPER_PASSWORD` | required | The Reddit account the app logs in as |
| `SCRAPER_SUBREDDITS` | built-in list | Comma-separated subreddits to scrape |
| `SCRAPER_USER_AGENT` | `content-automation-engine/0.1` | User agent sent to Reddit |
| `SCRAPER_POLL_INTERVAL` | `5s` | Minimum time between requests to Reddit |
| `LOG_FORMAT` | `text` | `text` or `json` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `HTTP_PORT` | `8000` | Port of the health endpoint |
| `PIPELINE_ENABLED` | `false` | Switches on scheduling, generation and posting |

Only read when `PIPELINE_ENABLED=true`:

| Variable | Default | Meaning |
|---|---|---|
| `SCHEDULER_INTERVAL` | required | How often a post is made, e.g. `6h` |
| `SCHEDULER_RUN_ON_START` | `false` | Also make a post as soon as the engine starts |
| `CONTENT_CLAIM_TTL` | `30m` | How long a selected story stays reserved |
| `CONTENT_MAX_ATTEMPTS` | `3` | Times a story may be selected before it is given up on |
| `CONTENT_MIN_BODY_CHARS`, `CONTENT_MAX_BODY_CHARS` | `300`, `2500` | Length of stories worth narrating |
| `GENERATOR_KOKORO_URL` | `http://localhost:8880` | Kokoro-FastAPI server (OpenAI-compatible `/v1/audio/speech`) |
| `GENERATOR_KOKORO_VOICE`, `GENERATOR_KOKORO_MODEL` | `af_heart`, `kokoro` | Voice and model |
| `GENERATOR_KOKORO_TIMEOUT` | `10m` | Limit for one narration request |
| `GENERATOR_OUTPUT_DIR` | system temp folder | Where videos are written before posting |
| `GENERATOR_MAX_NARRATION_CHARS` | `2500` | Cap on how much of a story is read |
| `GENERATOR_BACKGROUND_VIDEO` | none | Clip looped behind the captions; plain colour without it |
| `GENERATOR_BACKGROUND_COLOR` | `0x14171f` | Background colour when there is no clip |
| `GENERATOR_CAPTION_FONT`, `GENERATOR_CAPTION_FONT_SIZE` | `DejaVu Sans`, `84` | Caption style |
| `GENERATOR_WORDS_PER_CAPTION` | `5` | Words shown at once |
| `GENERATOR_FFMPEG_PATH`, `GENERATOR_FFPROBE_PATH` | `ffmpeg`, `ffprobe` | Binaries used to render |
| `POSTER_PLATFORM` | `dryrun` | `dryrun` logs instead of posting; `tiktok` posts for real |
| `POSTER_HASHTAGS` | `#reddit #redditstories #storytime` | Appended to every caption |
| `POSTER_MAX_CAPTION_CHARS` | `150` | Caption length limit |
| `POSTER_TIKTOK_SESSION_ID` | required for `tiktok` | The `sessionid` cookie of a logged-in TikTok session |
| `POSTER_TIKTOK_DATACENTER_ID` | none | The `tt-target-idc` cookie |
| `POSTER_TIKTOK_VISIBILITY` | `1` | `0` public, `1` private, `2` friends |

The TikTok client also reads its own older variables (`TIKTOK_LOGIN_URL`, `VIDEOS_DIR`, ...).

## Tests

Unit tests replace every outside system (MongoDB, Reddit, Kokoro, ffmpeg, TikTok) with in-memory
fakes from `incubator/internal/testutil`. From `incubator/`:

```bash
go test ./...
```

One test renders a real two-second video if `ffmpeg` is installed; `go test -short ./...` skips it.

The MongoDB queries themselves are checked by integration tests against a disposable server.
They are skipped unless `MONGO_TEST_URI` is set. Never point it at a real database:

```bash
docker run --rm -d --name engine-mongo-test -p 127.0.0.1:27018:27017 mongo:8.3.2
```

```bash
MONGO_TEST_URI=mongodb://127.0.0.1:27018 go test -tags integration ./internal/mongodb/integration/ ./internal/app/integration/
```

```bash
docker rm -f engine-mongo-test
```
