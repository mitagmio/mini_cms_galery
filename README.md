# sheyanova.art

Portfolio (Daria Sheyanova). **Admin CMS** on this server generates static HTML into `front/`, then publishes to **GitHub Pages**. The public site is not served from this host.

## Architecture (Phase 1)

```
Admin (React)  →  API (Go)  →  generate static  →  GitHub Pages
     │                │
     │                ├─ draft preview → /data/preview/  (served at /preview/)
     │                └─ publish       → FRONT_DIR (/front) + git push GHP
     │
api.sheyanova.art (this server)
  /admin/   React CMS
  /api/     CMS + media API
  /media/   uploads
  /preview/ static draft site
```

| Host | Where it lives |
|------|----------------|
| `https://www.sheyanova.art` (canonical) | GitHub Pages |
| `https://sheyanova.art` | GitHub Pages (redirect/alias to www) |
| `https://api.sheyanova.art` | This server (`sheyanova_nginx`) |

## Layout

```
front/          # generated static site (bind-mounted → API FRONT_DIR); published to GHP
api/            # Go CMS + generator + publish
admin/          # React admin → https://api.sheyanova.art/admin/
nginx-app/      # nginx for api.sheyanova.art (/api /media /admin /preview)
.envs/          # API/admin/web env (.env.api, .env.admin, .env.web)
```

## DNS checklist

1. **Apex + www → GitHub Pages** (not this VPS)
   - `sheyanova.art` and `www.sheyanova.art` → GitHub Pages (CNAME / A records per GitHub docs); do not point them at this host
   - Follow [GitHub Pages custom domain](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site)
   - Remove A/AAAA records that pointed apex/www at this server
   - Remove any Namecheap URL forward / old Format CNAME for www/apex
2. **API stays here**
   - `api.sheyanova.art` A (or AAAA) → this server (`sheyanova_nginx` via `.envs/.env.web`)
3. TLS
   - Site TLS: GitHub Pages / custom domain HTTPS
   - API TLS: nginx-proxy + acme-companion (`VIRTUAL_HOST` in `.envs/.env.web`) into shared `ci_certs`

## Publish env vars (`.envs/.env.api`)

| Variable | Purpose |
|----------|---------|
| `DATA_DIR` | CMS DB + runtime data (`/data`) |
| `UPLOAD_DIR` | Media uploads |
| `PREVIEW_DIR` | Draft static output (`/data/preview`) |
| `FRONT_DIR` | Publish output / GHP source tree (`/front`) |
| `CANONICAL_BASE` | Canonical public URL (`https://www.sheyanova.art`) |
| `GITHUB_REPO` | `owner/repo` for Pages |
| `GITHUB_BRANCH` | Branch GitHub Pages builds from (e.g. `gh-pages`) |
| `GH_TOKEN` | HTTPS push token (preferred simple path) |
| `GHP_SSH_KEY_PATH` | Alternative: path to deploy key inside container |

Compose mounts: `./front` → `/front`, `api-preview` → `/data/preview`, `api-ghp` → `/data/ghp`. Uncomment the SSH key volume in `docker-compose.yml` if using `GHP_SSH_KEY_PATH`.

## Phase 1 flow

1. Edit pages/blocks/media in **Admin** (`/admin/`).
2. **Preview** — API generates into `PREVIEW_DIR`; open `https://api.sheyanova.art/preview/`.
3. **Publish** — API writes static site into `FRONT_DIR`, then pushes `GITHUB_REPO` / `GITHUB_BRANCH`.
4. Visitors hit **GitHub Pages** at `CANONICAL_BASE` (no runtime API on the public site).

## Run

```bash
docker network ls | grep shared-web

nano .envs/.env.api    # ADMIN_TOKEN, GITHUB_*, GH_TOKEN
nano .envs/.env.web    # LETSENCRYPT_EMAIL (api only)

docker compose up -d --build
```

- Admin: `https://api.sheyanova.art/admin/`
- Health: `https://api.sheyanova.art/health`
- Preview: `https://api.sheyanova.art/preview/`
- Public site: `https://www.sheyanova.art` (after GHP DNS + first publish)

## Theme notes

- Before|After comparison sliders on home
- Colors: bg `#f6f1f1`, text `#2f2f2f`, accent `#ff0000`
- Fonts: Bluu Next / Work Sans / Cousine
