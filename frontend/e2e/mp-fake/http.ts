/**
 * Request and response helpers of the fake Mercado Pago (see ./server.ts).
 */
import type { IncomingMessage, ServerResponse } from 'node:http';
import { BODY_LIMIT, isObject } from './store.ts';
import type { Json } from './store.ts';

export type Handler = (req: IncomingMessage, res: ServerResponse, param: string) => void | Promise<void>;
export type Route = [method: string, path: RegExp, handler: Handler];

export async function readBody(req: IncomingMessage): Promise<string> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req) {
    const buf = chunk as Buffer;
    size += buf.length;
    if (size > BODY_LIMIT) throw new Error('body too large');
    chunks.push(buf);
  }
  return Buffer.concat(chunks).toString('utf8');
}

export async function readJson(req: IncomingMessage): Promise<Json> {
  const raw = await readBody(req);
  if (raw.trim() === '') return {};
  const parsed: unknown = JSON.parse(raw);
  if (!isObject(parsed)) throw new Error('body must be a JSON object');
  return parsed;
}

export function sendJson(res: ServerResponse, status: number, body: unknown): void {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
}

export function sendError(res: ServerResponse, status: number, message: string): void {
  sendJson(res, status, { message, error: message, status });
}

export function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => `&#${String(c.charCodeAt(0))};`);
}
