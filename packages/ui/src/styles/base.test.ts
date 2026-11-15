import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const css = readFileSync(resolve(process.cwd(), 'src/styles/base.css'), 'utf8');

/** The selector list of the coarse-pointer rule that sets the 44px floor. */
function floorSelectors(): string {
  const match = /@media \(pointer: coarse\) \{\s*([^{]+)\{\s*min-height: 44px;/.exec(css);
  return match?.[1] ?? '';
}

describe('base.css, the touch-target floor', () => {
  it('leaves the switch and the checkbox at their own size: their label row is the tap target', () => {
    const selectors = floorSelectors();
    expect(selectors).toContain('button');
    expect(selectors).toContain(":not([data-day], [role='switch'], [role='checkbox'])");
  });
});
