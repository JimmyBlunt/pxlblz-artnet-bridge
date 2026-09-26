# Windows local PXLBLZ Studio

PXLBLZ IDE upstream commit `d685125b` includes a managed `dev:main`
coordinator that assumes Unix process tools such as `ps -axo` and `lsof`.
That coordinator is therefore not used for native-Windows project operation.

The supported project path is now automated and CI-verified.

## One-command preparation

From the bridge repository:

```powershell
powershell -ExecutionPolicy Bypass -File ".\pxlblz-integration\prepare-windows-local-studio.ps1" `
  -PxlblzPath "..\PXLBLZ-IDE" `
  -MainPxlblzPath "..\PXLBLZ-IDE-main"
```

The helper:

1. verifies the clean worktree is really on `main`;
2. creates `.dev.vars` from the upstream example when needed;
3. generates a long random `SESSION_SECRET` if the placeholder is still present;
4. writes both worktree `.dev.vars` files as UTF-8 **without BOM**;
5. applies all local D1 migrations;
6. provisions the synthetic `github:local-dev` identity;
7. mints a PXLBLZ developer session;
8. writes the cookie to `.pxlblz-local-session.txt`;
9. prints the commands required to start Studio.

The UTF-8/no-BOM rule is important on Windows PowerShell 5.1. A BOM on the
first line can make Cloudflare see a different first binding name while
PXLBLZ's own text parser still trims it.

## Start the modified PXLBLZ worktree

```bat
cd /d "C:\path\to\PXLBLZ-IDE"
npm run dev
```

Default local URL:

```text
http://localhost:5174/PXLBLZ-IDE/
```

## Sign in with the synthetic developer session

The helper prints a browser-console command of the form:

```js
document.cookie = "pxlblz_session=<TOKEN>; path=/; SameSite=Lax"
```

Run it in DevTools on the localhost PXLBLZ tab, then verify:

```js
fetch('/api/me').then(r => r.json()).then(console.log)
```

Expected:

```text
authenticated: true
user.id: github:local-dev
```

Open Studio with output enabled:

```text
http://localhost:5174/PXLBLZ-IDE/studio?pxout=1
```

The output flag is retained in tab-local `sessionStorage`, so SPA navigation
and auth redirects in that tab do not silently disable it. Use `?pxout=0` to
turn it off again.

## Verified Windows regression gate

GitHub Actions now verifies the complete path on `windows-latest` against the
exact pinned PXLBLZ upstream commit:

```text
checkout pinned upstream
→ npm ci
→ apply integration installer
→ npm run build
→ git diff --check
→ prepare local D1 + local-dev identity
→ mint session
→ start ordinary npm run dev
→ GET /api/me signed out = false
→ GET /api/me with exact Cookie header = true
→ user.id = github:local-dev
```

This gate passed on 2026-09-26.

## OAuth

Real local GitHub or Google OAuth is not required for this Art-Net development
workflow. It can still be configured separately with local OAuth applications
if desired later.

## Dependency warning

Do **not** run:

```text
npm audit fix --force
```

against the pinned upstream checkout. It changes the tested dependency graph.
If dependencies are damaged, restore `package.json` and `package-lock.json`,
remove `node_modules`, and run `npm ci`.
