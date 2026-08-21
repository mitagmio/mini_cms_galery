# Sheyanova CMS API (Phase 1)

Go service: SQLite CMS + static site generator. Pure Go SQLite via `modernc.org/sqlite` (no CGO).

## Env

| Variable | Default | Notes |
|----------|---------|--------|
| `LISTEN_ADDR` | `:8080` | |
| `DATA_DIR` | `./data` | SQLite at `$DATA_DIR/cms.db` |
| `UPLOAD_DIR` | `$DATA_DIR/uploads` | Served at `/media/` |
| `PREVIEW_DIR` | `$DATA_DIR/preview` | Generated draft site |
| `FRONT_THEME_SRC` | (auto `../front`) | Copies `assets/`, `static/`, `fonts/`; also source for import-front |
| `IMPORT_FRONT` | auto | `1`/`0` force on/off; unset = import on boot when front has content pages (idempotent) |
| `ADMIN_TOKEN` | `change-me` | Bearer for `/api/admin/*` |
| `PREVIEW_BASE_URL` | `/preview` | Returned preview URLs |
| `GITHUB_TOKEN` / `GITHUB_REPO` | unset | Publish git push; stubbed if unset |

## Run locally

```bash
cd api
go run ./cmd/server
# or
CGO_ENABLED=0 go build -o server ./cmd/server
DATA_DIR=./data FRONT_THEME_SRC=../front ADMIN_TOKEN=dev ./server
```

## Curl smoke test

```bash
export TOKEN=dev BASE=http://127.0.0.1:8080

curl -s $BASE/health
curl -s -H "Authorization: Bearer $TOKEN" $BASE/api/admin/settings
curl -s -H "Authorization: Bearer $TOKEN" $BASE/api/admin/pages
curl -s -H "Authorization: Bearer $TOKEN" $BASE/api/admin/nav

# replace full nav tree (dropdown + links). PUT regenerates /preview (not GitHub publish).
# Body may be {"nav":[...]} or a raw array. kind=link | category. Omitted href + page_id → slug path.
curl -s -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -X PUT \
  -d '{"nav":[{"label":"BEAUTY","kind":"category","visible":true,"children":[{"label":"EDITORIAL I","kind":"link","page_id":"PAGE_ID","visible":true}]},{"label":"ABOUT","kind":"link","page_id":"ABOUT_ID","visible":true}]}' \
  $BASE/api/admin/nav

# replace blocks on a page
PID=$(curl -s -H "Authorization: Bearer $TOKEN" $BASE/api/admin/pages | jq -r '.pages[]|select(.is_homepage)|.id')
curl -s -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X PUT -d '{"blocks":[{"type":"comparison_slider","data":{"before_url":"/assets/cdn/004_before.jpg","after_url":"/assets/cdn/001_aaaaaaaaaa.jpg"}}]}' \
  $BASE/api/admin/pages/$PID/blocks

curl -s -H "Authorization: Bearer $TOKEN" -X POST $BASE/api/admin/preview/$PID
curl -s -H "Authorization: Bearer $TOKEN" -X POST $BASE/api/admin/generate
curl -s -H "Authorization: Bearer $TOKEN" -X POST -d '{"note":"manual"}' -H 'Content-Type: application/json' $BASE/api/admin/publish
curl -s -H "Authorization: Bearer $TOKEN" $BASE/api/admin/publish/history

# import blocks + media from static front/ (skips pages that already have blocks; ?force=1 replaces)
curl -s -H "Authorization: Bearer $TOKEN" -X POST "$BASE/api/admin/import-front"
curl -s -H "Authorization: Bearer $TOKEN" -X POST "$BASE/api/admin/import-front?force=1"
curl -s -H "Authorization: Bearer $TOKEN" -X POST $BASE/api/admin/generate
# check /preview/index.html for comparison_slider and /preview/editorial/ for gallery images

# media
curl -s -H "Authorization: Bearer $TOKEN" -F file=@photo.jpg -F title='shot' $BASE/api/admin/media
```

Public `/api/sliders*` is removed. Preview HTML is also served at `$BASE/preview/`.

### Import front → CMS

Idempotent importer reads HTML under `FRONT_THEME_SRC` / `FRONT_DIR` and fills BA + gallery pages (and contact with `?force=1`).

```bash
# boot: IMPORT_FRONT=1 DATA_DIR=./data FRONT_THEME_SRC=../front ADMIN_TOKEN=dev ./server
# or HTTP after start:
curl -s -H "Authorization: Bearer $TOKEN" -X POST "$BASE/api/admin/import-front"
```
