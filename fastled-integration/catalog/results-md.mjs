// Writes catalog/CATALOG_RESULTS.md from the catalog object (used by build-catalog.mjs).
import fs from 'node:fs';

export function writeResultsMd(cat, file) {
  const ex = cat.examples;
  const s = cat.summary;
  const catTitle = Object.fromEntries(cat.categories.map((c) => [c.id, c.titleDe]));
  const esc = (t) => String(t ?? '').replace(/\|/g, '\\|').replace(/\r?\n/g, ' ');
  const parityCell = (p) => !p ? '-' : p.status === 'identical' ? '0' : p.status === 'diff' ? `**L1 ${p.l1DiffBytes} / L2 ${p.l2DiffBytes} / raw ${p.rawDiffBytes} / meta ${p.metaDiffs}**` : p.status;
  const L = [];
  L.push('# FastLED-Beispielkatalog: Ergebnisse (generiert von `node catalog/build-catalog.mjs`)', '');
  L.push(`Stand: ${cat.generatedAt} | FastLED ${cat.fastled.version} @ ${cat.fastled.commit.slice(0, 10)} | zig ${cat.toolchain.zig} | ${cat.toolchain.frames} Frames, Seed ${cat.toolchain.seed}, Zeitplan wie \`npm run verify\``, '');
  L.push('Pro Beispiel: Kompilieren über den Compile-Dienst (`POST /compile`, wie die IDE), dann ' +
    `${cat.toolchain.frames} Frames in Node über \`runtime/fastledWasmHost.ts\`; wo es läuft zusätzlich native Referenz (x86_64, Win32-Fiber, ` +
    'FastLEDs eager Encode-Pfad) und Byte-Vergleich L1 (`leds[]`), L2 (Leitungsbytes RGB) und L2 roh pro Frame.', '');
  L.push('## Übersicht', '');
  L.push('| | Anzahl |', '|---|---:|');
  L.push(`| Beispiele gesamt | ${s.total} |`);
  L.push(`| Kompiliert | ${s.compileOk} |`, `| Kompilierfehler | ${s.compileError} |`);
  for (const [k, v] of Object.entries(s.run)) if (v) L.push(`| Lauf: ${k} | ${v} |`);
  L.push(`| Plattform pc (zeigt im PC-Modus etwas Sinnvolles) | ${s.platform.pc} |`, `| Plattform hardware | ${s.platform.hardware} |`);
  L.push(`| Parität nativ = wasm (0 abweichende Bytes) | ${s.parity.identical} |`, `| Parität mit Abweichung | ${s.parity.diff} |`);
  if (s.parity.other) L.push(`| Parität nicht messbar (Referenz-Build/-Lauf) | ${s.parity.other} |`);
  L.push('');
  L.push('## Nach Kategorie', '');
  L.push('| Kategorie | gesamt | kompiliert | läuft (ok/budget) | pc | hardware | Parität 0 |', '|---|---:|---:|---:|---:|---:|---:|');
  for (const c of cat.categories) {
    const xs = ex.filter((e) => e.category === c.id);
    if (!xs.length) continue;
    const n = (f) => xs.filter(f).length;
    L.push(`| ${c.titleDe} | ${xs.length} | ${n((e) => e.compile.status === 'ok')} | ${n((e) => ['ok', 'budget'].includes(e.run?.status))} | ` +
      `${n((e) => e.platform === 'pc')} | ${n((e) => e.platform === 'hardware')} | ${n((e) => e.parity?.status === 'identical')} |`);
  }
  L.push('');
  const failing = ex.filter((e) => e.compile.status !== 'ok' || !['ok', 'budget'].includes(e.run?.status) || (e.parity && e.parity.status !== 'identical'));
  L.push('## Fehlschläge und Abweichungen', '');
  if (!failing.length) L.push('Keine.');
  else {
    L.push('| Beispiel | Kategorie | Status | Grund |', '|---|---|---|---|');
    for (const e of failing) {
      const st = e.compile.status !== 'ok' ? 'Kompilierfehler' : !['ok', 'budget'].includes(e.run?.status) ? `Lauf: ${e.run?.status}` : `Parität: ${e.parity.status}`;
      const auto = e.compile.status !== 'ok' ? (e.compile.errors || []).slice(0, 2).map((d) => `${d.file ? d.file + ':' + d.line + ': ' : ''}${d.message}`).join(' / ')
        : e.run?.error || (e.run?.status === 'no-leds' ? 'keine LEDs registriert / kein show()' : e.parity ? parityCell(e.parity) + (e.parity.error ? ' ' + e.parity.error : '') : '');
      L.push(`| ${e.upstreamPath} | ${catTitle[e.category]} | ${st} | ${esc(e.reason ? e.reason + ' — ' : '')}${esc(auto).slice(0, 400)} |`);
    }
  }
  L.push('');
  L.push('## Alle Beispiele', '');
  L.push('| Beispiel | Kategorie | Kompilieren | Lauf | LEDs | Geometrie | UI | Plattform | Parität (Bytes) | Hinweis |', '|---|---|---|---|---:|---|---:|---|---|---|');
  for (const e of ex) {
    const g = e.geometry || {};
    const geo = g.dim === 2 ? (g.width ? `2D ${g.width}×${g.height}` : `2D frei (${g.shape ?? ''})`) : g.dim === 1 ? '1D' : g.dim === 3 ? '3D' : '-';
    L.push(`| ${e.upstreamPath} | ${catTitle[e.category]} | ${e.compile.status} | ${e.run?.status ?? '-'}${e.run?.budgetFrames ? ` (${e.run.budgetFrames} Budget-Frames)` : ''} | ` +
      `${e.ledCount ?? '-'} | ${geo} | ${e.uiElements?.length || 0} | ${e.platform} | ${parityCell(e.parity)} | ${esc(e.note)} |`);
  }
  L.push('');
  fs.writeFileSync(file, L.join('\n'), 'utf8');
}
