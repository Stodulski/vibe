#!/usr/bin/env node
/**
 * Stands in for the API while the Lighthouse job measures the built frontend.
 *
 * `vite preview` proxies /api to localhost:8080 (vite.config.ts, apiProxy). With
 * nothing listening there the proxy answers 502, and the client retries the
 * session check three times before it renders the auth routes, so the
 * measurement would be of a visitor who never exists. An anonymous visitor in
 * production gets a fast 401 from the backend instead. This answers the two
 * requests the login and register routes make on load the same way: the
 * `Unauthenticated` problem from backend/internal/httpx (errors.go,
 * InvalidAuthenticationToken), once for /auth/me and once for each refresh
 * attempt. Anything else under /api answers 404.
 *
 * usage: node scripts/lighthouse-api-stub.mjs   (from frontend/, port 8080)
 */
import { createServer } from 'node:http';

const PORT = Number(process.env.PORT ?? 8080);
const PROBLEM_BASE = 'https://vibe.com.ar/problems/';

function sendProblem(req, res, status, kind, title, detail, instance) {
  const headers = { 'Content-Type': 'application/problem+json', 'Cache-Control': 'no-store' };
  if (status === 401) headers['WWW-Authenticate'] = 'Bearer';
  res.writeHead(status, headers);
  res.end(
    JSON.stringify({ type: PROBLEM_BASE + kind, title, status, detail, instance, request_id: 'lighthouse-stub' }),
  );
  console.log(`${req.method} ${instance} -> ${status}`);
}

const ANONYMOUS_ROUTES = new Set(['GET /api/v1/auth/me', 'POST /api/v1/auth/refresh']);

const server = createServer((req, res) => {
  // Drain any body so the connection is reusable and the response is not held.
  req.resume();
  const path = new URL(req.url ?? '/', 'http://localhost').pathname;
  if (ANONYMOUS_ROUTES.has(`${req.method} ${path}`)) {
    sendProblem(req, res, 401, 'unauthorized', 'Unauthorized', 'invalid or missing authentication token', path);
    return;
  }
  sendProblem(req, res, 404, 'not-found', 'Not Found', 'no route in the Lighthouse API stub', path);
});

server.listen(PORT, () => {
  console.log(`lighthouse api stub listening on :${PORT}`);
});
