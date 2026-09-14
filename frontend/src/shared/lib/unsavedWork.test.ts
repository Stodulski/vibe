// @vitest-environment node
import { describe, it, expect, beforeEach } from 'vitest';
import { hasUnsavedWork, markUnsavedWork } from './unsavedWork';

describe('unsavedWork', () => {
  beforeEach(() => {
    markUnsavedWork('a', false);
    markUnsavedWork('b', false);
  });

  it('reports nothing unsaved when no form has registered', () => {
    expect(hasUnsavedWork()).toBe(false);
  });

  it('reports unsaved work while a form is dirty, and not after it goes clean', () => {
    markUnsavedWork('a', true);
    expect(hasUnsavedWork()).toBe(true);

    markUnsavedWork('a', false);
    expect(hasUnsavedWork()).toBe(false);
  });

  // Ids rather than a counter: a repeated mark for the same form (a re-run
  // effect, React StrictMode) must not leave the app permanently "dirty".
  it('counts the same form once, however many times it marks itself', () => {
    markUnsavedWork('a', true);
    markUnsavedWork('a', true);

    markUnsavedWork('a', false);

    expect(hasUnsavedWork()).toBe(false);
  });

  it('stays dirty while any other form still is', () => {
    markUnsavedWork('a', true);
    markUnsavedWork('b', true);

    markUnsavedWork('a', false);

    expect(hasUnsavedWork()).toBe(true);
  });
});
