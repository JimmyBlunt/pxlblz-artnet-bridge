# Windows local PXLBLZ Studio workaround

PXLBLZ IDE upstream commit `d685125b` has a managed `dev:main` runtime
coordinator that assumes Unix process-management tools.

On native Windows the coordinator calls commands such as:

```text
ps -axo pid=,pgid=,command=
lsof -nP -iTCP:<port> -sTCP:LISTEN -t
```

The Windows `ps.exe` does not support those arguments, so `npm run dev:main`
can time out waiting for `http://localhost:5174/api/me` even though the core
Vite/Cloudflare development stack itself works.

For this project, use the normal Vite worker runtime directly on Windows and
prepare the local D1 manually.

## Art-Net worktree

The modified worktree is the one containing the experimental
`externalPixelOutput.ts` integration.

Copy the shared development variables into it, migrate its own local D1, seed
the synthetic developer identity, then run ordinary Vite:

```bat
copy /Y "..\PXLBLZ-IDE-main\.dev.vars" ".dev.vars"

npm run db:migrate:local

npx wrangler d1 execute pxlblz-ide --local --command "INSERT INTO users (id, github_user_id, github_login, display_name, avatar_url, created_at, updated_at) VALUES ('github:local-dev','local-dev','local-dev','Local Dev',NULL,unixepoch(),unixepoch()) ON CONFLICT(id) DO UPDATE SET display_name=excluded.display_name, updated_at=excluded.updated_at;"

npm run dev
```

The Cloudflare Vite plugin serves the UI, Worker API and local D1 in the same
process. Its default local URL is:

```text
http://localhost:5174/PXLBLZ-IDE/
```

## Synthetic local session

Once a clean `main` worktree exists, the session helper can still be used on
Windows because the developer-session path does not require the Unix process
coordinator:

```bat
npm run dev:session -- --developer
```

It prints:

```text
pxlblz_session=<token>
```

Set that cookie for localhost, reload, and open:

```text
http://localhost:5174/PXLBLZ-IDE/studio?pxout=1
```

This keeps OAuth credentials unnecessary for local Art-Net testing.

Do not run `npm audit fix --force` in the upstream checkout. The project pins
specific alpha/dev dependencies and forced audit remediation can change the
tested dependency graph.
