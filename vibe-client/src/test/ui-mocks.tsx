import { createElement, type ComponentPropsWithoutRef, type ReactElement } from 'react';

/**
 * Host tags used by the shadcn/Radix component mocks across `*.test.*` files.
 * Extend this union if a new mock needs a tag not listed here.
 */
type PassthroughTag = 'button' | 'input' | 'label' | 'div' | 'option' | 'span';

/**
 * Builds a typed "dumb wrapper" stub for a shadcn/Radix UI component mock.
 *
 * Used inside `vi.mock('@/shared/components/ui/...', () => ({ Button: passthrough('button') }))`
 * to replace shadcn components with a plain host element in unit tests, without
 * resorting to `any` for the mock's prop type. All received props (including
 * `children`) are spread onto the given host tag.
 *
 * Gotcha: `vi.mock` factories are hoisted above imports by Vitest. Importing
 * `passthrough` at the top of a test file works for STATIC `vi.mock` factories
 * (the import binding is live by the time the factory actually executes), but
 * if a factory needs to reference other test-only helpers that themselves
 * import from application code, prefer the dynamic-import form:
 * `vi.mock(path, async () => { const { passthrough } = await import('@/test/ui-mocks'); ... })`
 * to avoid any hoisting-order surprises at collection time.
 */
export function passthrough<E extends PassthroughTag>(tag: E) {
  return function Passthrough(props: ComponentPropsWithoutRef<E>): ReactElement {
    return createElement(tag, props);
  };
}
