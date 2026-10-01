// Import or update a PXLBLZ custom map and/or pattern in the LOCAL PXLBLZ
// database (Cloudflare D1 under PXLBLZ-IDE-main/.wrangler/state) for the local
// developer user, exactly as the Studio stores them: the map is baked with
// PXLBLZ's own bakeMapSource(); the pattern is linked to the map via
// settings_json.mapId / pixelCount.
//
// Run from the PXLBLZ-IDE-main folder (needs its node_modules for tsx/wrangler):
//   npx tsx <bridge>/pxlblz-integration/tools/local-d1-import.mts \
//       --map <bridge>/pxlblz-integration/maps/esp-test-8x8-12x6.js --map-name "ESP Testrig" --pixels 136 \
//       --pattern <bridge>/pxlblz-integration/patterns/snowflake-icesparkle-carpet-v06-esp-test.js \
//       --pattern-name "SnowFlake IceSparkle Carpet v0.6b - ESP Testrig" --out import.sql
//   npx wrangler d1 execute pxlblz-ide --local --file import.sql
//
// Updating instead of inserting: --map-id <id> / --pattern-id <id> (ids are printed on insert).
// Back up .wrangler/state first. Reload the Studio tab (F5) afterwards.
import { readFileSync, writeFileSync } from 'node:fs'
import { randomUUID } from 'node:crypto'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

function arg(name: string): string | undefined {
  const i = process.argv.indexOf(`--${name}`)
  return i >= 0 ? process.argv[i + 1] : undefined
}

const USER = arg('user') ?? 'github:local-dev'
const ide = path.resolve(arg('ide') ?? process.cwd())
const out = arg('out') ?? 'import.sql'
const q = (s: string) => "'" + s.replace(/'/g, "''") + "'"

const { bakeMapSource } = await import(pathToFileURL(path.join(ide, 'src/engine/maps/bake.ts')).href)

const now = Date.now()
const nowS = Math.floor(now / 1000)
const sql: string[] = []
let mapId = arg('map-id')

const mapFile = arg('map')
if (mapFile) {
  const pixels = Number(arg('pixels'))
  if (!Number.isInteger(pixels) || pixels <= 0) throw new Error('--pixels <count> is required with --map')
  const source = readFileSync(mapFile, 'utf8')
  const baked = bakeMapSource(source, pixels)
  if (baked.points.length !== pixels) throw new Error(`map produced ${baked.points.length} points, expected ${pixels}`)
  const grid = baked.gridDims ? q(JSON.stringify(baked.gridDims)) : 'NULL'
  if (mapId) {
    sql.push(`UPDATE personal_maps SET source=${q(source)}, points_json=${q(JSON.stringify(baked.points))}, grid_dims_json=${grid}, updated_at=${now} WHERE user_id=${q(USER)} AND id=${q(mapId)};`)
  } else {
    mapId = randomUUID()
    const name = arg('map-name') ?? path.basename(mapFile, path.extname(mapFile))
    sql.push(`INSERT INTO personal_maps (user_id,id,name,dim,generator,params_json,points_json,source,grid_dims_json,created_at,updated_at) VALUES (${q(USER)},${q(mapId)},${q(name)},${baked.dim},'custom','{}',${q(JSON.stringify(baked.points))},${q(source)},${grid},${nowS},${now});`)
    sql.push(`UPDATE personal_settings SET value_json = json_insert(value_json,'$.nodes[#]',json('{"kind":"entity","entityId":"${mapId}"}')), updated_at=${now} WHERE user_id=${q(USER)} AND key='mapOrganization';`)
  }
  console.log(`map ${mapId}: ${baked.points.length} points, dim ${baked.dim}`)
}

const patternFile = arg('pattern')
if (patternFile) {
  const src = readFileSync(patternFile, 'utf8')
  let patternId = arg('pattern-id')
  const name = arg('pattern-name')
  if (patternId) {
    sql.push(`UPDATE personal_patterns SET src=${q(src)}${name ? `, name=${q(name)}` : ''}, updated_at=${now} WHERE user_id=${q(USER)} AND id=${q(patternId)};`)
  } else {
    patternId = randomUUID()
    const settings: Record<string, unknown> = { brightness: 1 }
    if (mapId) settings.mapId = mapId
    if (arg('pixels')) settings.pixelCount = Number(arg('pixels'))
    sql.push(`INSERT INTO personal_patterns (user_id,id,name,src,controls_json,settings_json,created_at,updated_at) VALUES (${q(USER)},${q(patternId)},${q(name ?? path.basename(patternFile, path.extname(patternFile)))},${q(src)},'{}',${q(JSON.stringify(settings))},${nowS},${now});`)
    sql.push(`UPDATE personal_settings SET value_json = json_insert(value_json,'$.nodes[#]',json('{"kind":"entity","entityId":"${patternId}"}')), updated_at=${now} WHERE user_id=${q(USER)} AND key='patternOrganization';`)
    sql.push(`UPDATE personal_settings SET value_json = ${q(JSON.stringify({ type: 'pattern', id: patternId }))}, updated_at=${now} WHERE user_id=${q(USER)} AND key='lastActive';`)
  }
  console.log(`pattern ${patternId}`)
}

if (!sql.length) throw new Error('nothing to do: pass --map and/or --pattern')
writeFileSync(out, sql.join('\n') + '\n')
console.log(`SQL written to ${out} - apply with: npx wrangler d1 execute pxlblz-ide --local --file ${out}`)
