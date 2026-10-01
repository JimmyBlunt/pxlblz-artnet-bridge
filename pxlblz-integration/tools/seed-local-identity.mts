// Write the SQL that provisions PXLBLZ's local developer identities
// (github:local-dev and the local agent users) with PXLBLZ's own
// localIdentitySeedSql() - the documented way to make the local Studio login
// work on a new PC (see windows-launcher-artnet/README.md, step 5).
//
// Run from PXLBLZ-IDE-main after `npm run db:migrate:local`:
//   npx tsx <bridge>/pxlblz-integration/tools/seed-local-identity.mts seed.sql
//   npx wrangler d1 execute pxlblz-ide --local --file seed.sql
import { readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

const ide = process.cwd()
const out = process.argv[2] ?? 'seed-local-identity.sql'
const { localIdentitySeedSql } = await import(pathToFileURL(path.join(ide, 'scripts/dev-runtime-auth.ts')).href)
const manifest = JSON.parse(readFileSync(path.join(ide, 'dev-runtime.json'), 'utf8'))
writeFileSync(out, localIdentitySeedSql(manifest))
console.log(`seed SQL written to ${out} - apply with: npx wrangler d1 execute pxlblz-ide --local --file ${out}`)
