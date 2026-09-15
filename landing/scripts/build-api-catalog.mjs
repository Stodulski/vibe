/**
 * Pre-build pass that regenerates public/.well-known/api-catalog — the RFC 9727
 * catalog an agent reads to discover the Vibe API without being told its URLs
 * beforehand. Astro copies it into dist/ like any other public asset.
 *
 * It is DERIVED from backend/internal/openapi/openapi.yaml rather than written
 * by hand, for the same reason build-markdown.mjs derives llms.txt from the
 * built HTML: a hand-kept copy of URLs that live somewhere else goes stale the
 * first time somebody renames one, and nothing fails. The catalog then lies to
 * every agent that reads it, which is worse than publishing no catalog at all.
 *
 * The spec is the source of truth for two things: the production server URL
 * (its `servers` list) and the existence of every endpoint advertised below.
 * Rename /api/v1/docs in the backend and this build fails naming it, instead of
 * shipping a catalog that points at a 404.
 *
 * The generated file is committed so the landing still builds where backend/ is
 * not part of the checkout: this project's Vercel Root Directory is landing/,
 * and whether files outside it reach the build step is a dashboard setting this
 * repository does not control. When the spec IS readable the file is rewritten
 * and drift fails the build; when it is not, the committed copy ships and the
 * build says so out loud.
 */
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { parse } from 'yaml';

const SPEC = new URL('../../backend/internal/openapi/openapi.yaml', import.meta.url).pathname;
const OUT_DIR = new URL('../public/.well-known/', import.meta.url).pathname;
const OUT = `${OUT_DIR}api-catalog`;
const SITE = 'https://vibe.com.ar';

/* What the catalog advertises, in the order it is written. Each entry names a
   path that MUST exist in the spec as a GET — that assertion is the whole point
   of generating this file instead of keeping it by hand. */
const ADVERTISED = [
  { rel: 'service-desc', path: '/api/v1/openapi.json', type: 'application/openapi+json', title: 'OpenAPI 3.1 description (JSON)' },
  { rel: 'service-desc', path: '/api/v1/openapi.yaml', type: 'application/openapi+yaml', title: 'OpenAPI 3.1 description (YAML)' },
  { rel: 'service-doc', path: '/api/v1/docs', type: 'text/html', title: 'Interactive reference' },
  { rel: 'status', path: '/api/v1/healthcheck', type: 'application/json', title: 'Health check' },
];

let spec;
try {
  spec = parse(readFileSync(SPEC, 'utf8'));
} catch (err) {
  if (err.code !== 'ENOENT') throw err;
  console.log('  api-catalog  backend/ is not in this checkout — shipping the committed copy');
  process.exit(0);
}

/* The first https server is production. localhost is also listed, and a catalog
   published at vibe.com.ar pointing an agent at localhost would be nonsense. */
const base = (spec.servers ?? []).map(s => s.url).find(u => u?.startsWith('https://'));
if (!base) {
  throw new Error(`${SPEC}: no https server in its "servers" list — the catalog has no base URL to advertise`);
}

const missing = ADVERTISED.filter(({ path }) => !spec.paths?.[path]?.get).map(({ path }) => path);
if (missing.length > 0) {
  throw new Error(
    `${missing.join(', ')} — advertised by the API catalog but not a GET in the spec. ` +
    `Renamed or removed? Update ADVERTISED in scripts/build-api-catalog.mjs, or the catalog ships pointing at a 404.`
  );
}

const name = spec.info?.title ?? 'Vibe API';

const entry = { anchor: `${base}/api/v1` };
for (const { rel, path, type, title } of ADVERTISED) {
  (entry[rel] ??= []).push({ href: `${base}${path}`, type, title: `${name} - ${title}` });
}
entry.author = [{ href: SITE, title: 'Vibe' }];
entry.describedby = [{ href: `${SITE}/llms.txt`, type: 'text/plain', title: 'Vibe - plain-text overview for agents' }];

mkdirSync(OUT_DIR, { recursive: true });
writeFileSync(OUT, `${JSON.stringify({ linkset: [entry] }, null, 2)}\n`);
console.log(`  api-catalog  ${base}/api/v1 — ${ADVERTISED.length} links from ${spec.info?.version ?? 'the spec'}`);
