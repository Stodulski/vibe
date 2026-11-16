import { describe, expect, it } from 'vitest';
import { ES_AR } from './es_AR';

const LONG_DASH = /[–—]/;

/** Every user-visible string in the tree, with its path; functions are checked through their source. */
function collect(node: unknown, path: string, out: { path: string; text: string }[]): void {
  if (typeof node === 'string') out.push({ path, text: node });
  else if (typeof node === 'function') out.push({ path, text: node.toString() });
  else if (Array.isArray(node))
    node.forEach((item, i) => {
      collect(item, `${path}[${String(i)}]`, out);
    });
  else if (node && typeof node === 'object') {
    for (const [key, value] of Object.entries(node)) collect(value, `${path}.${key}`, out);
  }
}

describe('es_AR copy', () => {
  it('never uses an em dash or an en dash (use a period, comma, colon, parentheses or "a" for ranges)', () => {
    const all: { path: string; text: string }[] = [];
    collect(ES_AR, 'ES_AR', all);
    expect(all.length).toBeGreaterThan(100);
    const offenders = all.filter(({ text }) => LONG_DASH.test(text)).map(({ path }) => path);
    expect(offenders).toEqual([]);
  });
});
