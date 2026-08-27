# CMS Architecture — Phase 1 MVP

Locked plan for Sheyanova Format-like constructor. Public site is static-only; CMS lives with Go API + React admin on `api.sheyanova.art`.

---

## Locked decisions

| # | Decision |
|---|----------|
| 1 | Public site = static only → GitHub Pages (`sheyanova.art` / `www`). **Canonical:** `https://www.sheyanova.art` |
| 2 | Admin + API on `api.sheyanova.art` via `nginx-app`. Public site is GitHub Pages only (no site nginx on this host) |
| 3 | Go API owns CMS: SQLite `/data/cms.db` + uploads on disk; auth = `Authorization: Bearer ADMIN_TOKEN` |
| 4 | Page templates: `ba_content`, `panorama_gallery`, `text_content`, `lookbook_gallery`, `rates_content` |
| 5 | Blocks: `comparison_slider`, `gallery_image`, `rich_text`, `contact_form`, `rate_banner` |
| 6 | Generator in Go (`internal/generate`): draft → `/data/preview/` (all pages); publish → **published pages only** into `front/` + git push GHP repo |
| 7 | React admin: Format-like pages / media / settings / templates / preview / publish |
| 8 | Contact form v1 = **mailto** (API contact endpoint later) |
| 9 | Seed CMS from existing `front/` inventory |
| 10 | **Phase 1 MVP only** (no multi-user, no public API, no horizontal beauty strip product) |

```
[React Admin] ──Bearer──► [Go CMS API]
                              │ SQLite /data/cms.db
                              │ uploads /data/uploads/
                              ├── generate → /data/preview/
                              └── publish  → front/ ──git push──► GitHub Pages
Public ──► www.sheyanova.art (HTML/CSS/JS/images only; no /api)
```

---

## Data model (SQLite `/data/cms.db`)

### `site_settings` (singleton row)
- `canonical_base` = `https://www.sheyanova.art`
- `site_name`, `default_title_suffix`, `default_description`, `robots` (`noai, noimageai`)
- Logo text, favicon media IDs (16/32/96/192), `og_image_media_id`
- Theme tokens: bg `#f6f1f1`, text `#2f2f2f`, accent `#ff0000`
- Social URLs (Instagram / Behance / LinkedIn)
- GHP: `github_repo`, `github_branch` (default `main`)
- Contact: `mailto_address` (v1 form target)

### `templates` (page blueprints)
| Field | Notes |
|-------|--------|
| `kind` | `page` (generate engine) or `form` (named Rate overlay). Form rows are not page engines. |
| `form_key` | form templates only: `fashion` \| `beauty` \| `lookbook` \| `editorial` \| `product` \| `manual` |
| `id` | page system = theme key (`ba_content`…); form system = `form_<key>`; custom = generated id |
| `theme` / `key` | generate engine only: `ba_content` \| `panorama_gallery` \| `text_content` \| `lookbook_gallery` \| `rates_content`. Form templates store `theme=rates_content` without becoming engines. |
| `name` / `label`, `description` | admin display |
| `allowed_blocks` | JSON string array of block types (empty for form templates) |
| `default_blocks` | JSON array of `{type, data}` starters (empty for form templates) |
| `is_system` | built-ins; theme/id locked, metadata editable |
| `sort_order`, `created_at`, `updated_at` | |

### `pages`
| Field | Notes |
|-------|--------|
| `id`, `slug`, `title`, `nav_label` | slug `""` or special flag for homepage |
| `template` | `ba_content` \| `panorama_gallery` \| `text_content` \| `lookbook_gallery` \| `rates_content` |
| `status` | `draft` \| `published` (live site updates only on Publish) |
| `is_homepage` | exactly one; today = BA `/` |
| `meta_title`, `meta_description`, `canonical_path`, `og_image_media_id` | SEO overrides |
| `settings_json` | page extras (`{}` default). Lookbook stores `shuffle_seed`. Rates stores `banner_aspect` (`3:4` default) and optional `banner_min_height` (px) for the shared Rate banner grid |
| `sort`, `updated_at` | |

### `blocks` (ordered per page)
| Field | Notes |
|-------|--------|
| `id`, `page_id`, `position`, `type`, `data` (JSON) | |

| `type` | `data` sketch |
|--------|----------------|
| `comparison_slider` | `{ before_media_id, after_media_id, caption? }` |
| `gallery_image` | `{ media_id, alt, caption? }` |
| `rich_text` | `{ html }` |
| `contact_form` | `{ mailto, fields: [name,email,message] }` — static shell |
| `rate_banner` | `{ form_template_id, form_key, media_id, alt, caption, start_from_label, price, currency }` — Rates tiles. `form_template_id` (e.g. `form_fashion`) chooses the named form template; `form_key` is derived / fallback |

### `media`
- `id`, `filename`, `path` (under `/data/uploads/`), `mime`, `width`, `height`, `bytes`
- `title`, `alt`, `kind` optional (`before`/`after`/`portfolio`), `created_at`
- On generate: copy referenced files → `assets/cdn/…`; rewrite `src` to site-relative paths

### `nav`
- `id`, `parent_id`, `label`, `href`, `page_id`, `sort_order`, `kind` (`link` \| `category`), `visible`
- Tree: top-level `link` (simple item) or `category` (one-level dropdown; children are `link`s). Category `href` may be empty. Link with `page_id` and empty `href` is filled from the page slug.
- PUT `/api/admin/nav` replaces the full tree. Does **not** regenerate preview by default (optional `?generate=1` or `{"generate":true}`). Refresh draft via Generate draft / Preview. Does not publish to GitHub.
- Seed: BEAUTY → editorial children; top-level BEFORE\|AFTER, FASHION, EDITORIAL, PRODUCT, ABOUT, CONTACT + socials

### `publish_history`
- `id`, `created_at`, `status` (`ok`\|`failed`), `commit_sha`, `note`, `error`, counts

**Auth:** all `/api/admin/*` require Bearer `ADMIN_TOKEN`. No public CMS/API. CORS: admin origin + local Vite only.

---

## API routes (`https://api.sheyanova.art`)

| Method | Path | Auth | Notes |
|--------|------|------|--------|
| GET | `/health` | no | |
| GET | `/api/admin/me` | yes | token check |
| GET/PUT | `/api/admin/settings` | yes | |
| GET/PUT | `/api/admin/nav` | yes | nested tree; PUT replaces + regenerates preview |
| GET | `/api/admin/pages` | yes | list |
| POST | `/api/admin/pages` | yes | create |
| GET/PATCH/DELETE | `/api/admin/pages/{id}` | yes | |
| GET | `/api/admin/pages/{id}/blocks` | yes | ordered |
| PUT | `/api/admin/pages/{id}/blocks` | yes | full replace + reorder |
| GET | `/api/admin/media` | yes | list/filter |
| POST | `/api/admin/media` | yes | multipart |
| PATCH/DELETE | `/api/admin/media/{id}` | yes | |
| GET | `/media/{file}` | yes* | uploads for admin/preview (*Bearer or same-origin admin) |
| POST | `/api/admin/generate` | yes | draft → `/data/preview/` |
| POST | `/api/admin/preview/{pageId}` | yes | one page; return preview URL |
| GET | `/preview/…` | yes | static file server for draft tree |
| POST | `/api/admin/publish` | yes | generate → `front/` → git push; write history |
| GET | `/api/admin/publish/history` | yes | |
| POST | `/api/admin/seed` | yes | one-shot import from `front/` (MVP bootstrap) |
| GET | `/api/admin/templates` | yes | list page + form templates (system + custom) |
| POST | `/api/admin/templates` | yes | create custom **page** blueprint (`theme` = generate engine). Do not mint form engines as pages. |
| GET/PATCH/PUT | `/api/admin/templates/{id}` | yes | read / update metadata + default blocks |

**Remove:** `GET /api/sliders*`, old `/api/admin/photos`, `/api/admin/sliders`, slider-only preview.

---

## Disk layout

```
/data/                          # api-data volume
  cms.db
  uploads/                      # originals (api-uploads volume)
  preview/                      # draft generated site
  ghp-worktree/                 # persistent git clone for Pages push
  snapshots/                    # optional publish tarballs

/root/sheyanova/front/          # publish output mirror (theme kit + generated pages)
  .nojekyll
  CNAME                         # www.sheyanova.art
  index.html                    # homepage
  {slug}/index.html             # directory URLs only (no dual flat *.html in generator)
  assets/theme/…                # BA + panorama chrome (vendored)
  assets/cdn/…                  # published media copies
  static/…  fonts/…

.env.api
  ADMIN_TOKEN, DATA_DIR, UPLOAD_DIR, FRONT_DIR, PREVIEW_DIR
  GHP_REPO, GHP_BRANCH, GH_TOKEN|GHP_SSH_KEY_PATH
  GIT_AUTHOR_NAME/EMAIL
```

---

## Admin routes (`base: /admin/`)

| Route | Screen |
|-------|--------|
| `/login` | Paste Bearer token → `localStorage` |
| `/` | Dashboard: last publish, draft dirty, shortcuts |
| `/pages` | List / create / duplicate |
| `/pages/:id` | Three-pane editor: palette · canvas · inspector (block + SEO) |
| `/pages/:id/preview` | Desktop iframe → `/preview/{slug}/` |
| `/media` | Upload, kind filter, grid, picker modal |
| `/settings` | Site SEO, favicons, socials, mailto, GHP meta (read-only secrets) |
| `/templates` | Page blueprints + named form templates (Fashion, Beauty, …). Form templates cannot create a page. |
| `/preview` | Full-site draft iframe |
| `/publish` | Confirm → POST publish → poll status → SHA + live link |

Persistent nav: Dashboard · Pages · Media · Templates · Settings · Preview · Publish · Logout.

**Editor rules:** template constrains allowed blocks; Save = CMS only; Preview = generate draft; Publish = full site push.

---

## Generate / publish flow

1. **Save** — write pages/blocks/media to SQLite (live `front/` untouched).
2. **Generate (draft)** — `internal/generate` reads CMS + theme kits → `/data/preview/`; copy used media; emit chrome from `nav` + `settings`; serve at `https://api.sheyanova.art/preview/`.
3. **Publish** (mutex):
   - Generate into `FRONT_DIR` (`front/`) **and** sync into `ghp-worktree` (preserve `.git`)
   - Ensure `.nojekyll`, `CNAME` (`www.sheyanova.art`), theme assets
   - `git add -A` → commit → `git push`
   - Append `publish_history`; return `{ commit_sha, url }`
4. **Public** never hits API; all images are in published tree under `/assets/cdn/`.

### Template → HTML mapping

| Template | Shell / assets | Blocks |
|----------|----------------|--------|
| `ba_content` | BA chrome + comparison CSS/JS + `ba-harden` | N× `comparison_slider` |
| `panorama_gallery` | panorama theme.js + `gallery-harden` | N× `gallery_image` → `.asset.image` |
| `lookbook_gallery` | masonry grid (`lookbook.css`) + overlay panorama (`lookbook-harden`, **not** unscoped `gallery-harden`); wheel via parameterized `gallery-wheel.js` | N× `gallery_image` (empty skipped). Order: Fisher–Yates at generate with stored `settings.shuffle_seed` |
| `text_content` | content chrome (no panorama JS) | `rich_text`, `contact_form` |
| `rates_content` | text chrome + `rates.css` / `rates.js`; 3×2 tiles | `rich_text`, `rate_banner`. Each banner chooses a **form template** (`form_template_id`). Overlay forms POST `/api/contact` with `form=rates_*`. Shared grid size: page `settings.banner_aspect` (default `3:4`). Preview includes drafts; publish writes **published pages only** (omit draft `/rates/`). |

Canonical / og:url always under `https://www.sheyanova.art/…`.

---

## Seed inventory (from `front/`)

| Slug | Template | Seed content |
|------|----------|--------------|
| `/` (homepage) | `ba_content` | BA stack (= current `index.html` / before-after) |
| `before-after` | `ba_content` | same BA stack (or alias page → homepage; prefer one homepage + optional redirect page) |
| `editorial`, `editorial-i`, `editorial-ii`, `editorial-3`, `editorial-iv` | `panorama_gallery` | gallery assets |
| `fashion`, `product`, `about` | `panorama_gallery` | gallery assets |
| `contact` | `text_content` | rich_text + contact_form (mailto) |

Lookbook (`lookbook_gallery`) is a **template only** — not seeded as a live page or nav item. Create from Admin → Templates / Pages when needed.

Rates (`rates_content`, slug `rates`) is ensured on API boot as **draft**, with nav item RATES immediately before ABOUT (`visible=true` for preview). Publish omits it until status is flipped.

**MVP choice:** homepage = BA content; keep `before-after` as published page with same blocks **or** nav-only link to `/` — implement as single homepage page + nav label “BEFORE | AFTER”. Do not emit duplicate flat `*.html` aliases; clean Wayback junk `front/https:` out of generator inputs.

Import path: parse existing HTML → media rows (from `assets/cdn/`) + blocks → SQLite via `POST /api/admin/seed`.

---

## Deprecate list

| Item | Action |
|------|--------|
| Former site nginx / `sheyanova_front` | Removed — public site is GitHub Pages only |
| `.envs/.env.front` | Removed |
| Public `/api` + `/media` proxies on site nginx | Removed with former site nginx |
| `GET /api/sliders*` | Delete |
| Old admin photos/sliders-only UX | Replace with pages/blocks/media |
| CORS for `sheyanova.art` | Drop once public API gone |
| Format contact POST / CDN hotlinks | Generator uses local theme + mailto |
| Dual `slug.html` + `slug/index.html` | Generator: directory URLs only |
| `front/https:` dump tree | Ignore / delete; not seeded |

**Keep:** `nginx-app` (`/api`, `/media`, `/admin`, `/preview`), React `base: '/admin/'`, theme kits under `front/assets` + `static` + `fonts`, Docker volumes for `/data`.

**DNS (infra):** apex + www → GitHub Pages; `api.sheyanova.art` stays on this host. TLS: GHP for public, LE via nginx-proxy for API.

---

## Phase 1 MVP scope

**In**
- SQLite CMS + media uploads + Bearer auth
- Four templates, four block types
- Seed from current pages
- Generate draft preview + Publish to GHP
- Admin screens listed above
- Contact mailto
- Strip public slider API; public site served only via GitHub Pages

**Out**
- Multi-user / roles / cookie sessions
- Public contact API / SMTP
- Horizontal beauty-strip as product surface
- Nav WYSIWYG chrome, mobile preview lab
- Publish rollback UI (git history is enough; list SHAs in history)
- Image crop/folders/LFS
- Pixel-perfect Format `_4ORMAT_DATA` parity beyond what panorama JS needs

---

## Implementation order (parallel agents)

Ownership boundaries — **do not edit outside your tree** except shared contracts in this doc / OpenAPI sketch in `api` if needed.

### Wave 0 — contracts (any, short)
- Freeze this doc; add env keys to `.envs/.env.api.example` (infra can land stubs).

### Wave 1 — parallel

| Agent | Owns | Deliverables |
|-------|------|----------------|
| **api** | `api/` | SQLite schema + migrations; CRUD settings/nav/pages/blocks/media; Bearer middleware; seed importer; stub generate/publish handlers |
| **admin** | `admin/` | Route shell + login; pages list/editor (mock OK until API ready); media library; settings/templates/preview/publish screens wired to `/api/admin/*` |
| **infra** | `docker-compose.yml`, `nginx-app/`, `.envs/` | API/admin only in compose; `/preview/` proxy; volumes for preview + ghp-worktree; env for GHP; document DNS cutover |

### Wave 2 — api (after schema)
- `internal/generate`: templates from Format shells; draft → `/data/preview/`; publish → `front/` + git push; `publish_history`
- Serve `/preview/` with directory `index.html` try_files behavior

### Wave 3 — integrate
- admin: real preview iframe + publish polling
- infra: GHP deploy key/PAT on host; DNS tip sheet in README
- api: run seed once; smoke generate

### Wave 4 — cutover
- Publish first GHP build; point www/apex DNS to Pages
- Confirm site nginx artifacts gone; remove leftover slider endpoints
- Confirm public site has **zero** calls to `api.sheyanova.art` except intentional later contact API

### Agent interface contract
- Admin talks only to `/api/admin/*` + `/preview/` + `/media/`
- Generator reads only SQLite + `/data/uploads` + theme files under `FRONT_DIR` kit paths
- Infra never invents CMS schema; only mounts paths and proxies

---

## Env checklist (`.envs/.env.api`)

```
LISTEN_ADDR=:8080
DATA_DIR=/data
UPLOAD_DIR=/data/uploads
FRONT_DIR=/front
PREVIEW_DIR=/data/preview
ADMIN_TOKEN=<long-random>
GHP_REPO=git@github.com:ORG/sheyanova.art.git
GHP_BRANCH=main
GH_TOKEN=…   # or GHP_SSH_KEY_PATH=
CORS_ORIGINS=https://api.sheyanova.art,http://localhost:5173
MAX_UPLOAD_MB=25
```

Compose: mount `./front` → `/front` on API for publish output; keep admin + nginx-app only for public interactive surface.
