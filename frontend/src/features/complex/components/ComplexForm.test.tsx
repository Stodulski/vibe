import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import './complex-form-test-mocks';
import { ComplexForm } from './ComplexForm';

describe('ComplexForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders name input', () => {
    renderWithProviders(<ComplexForm />);
    expect(screen.getByLabelText(/nombre/i)).toBeInTheDocument();
  });

  it('renders slug input', () => {
    renderWithProviders(<ComplexForm />);
    expect(screen.getByLabelText(/slug|url/i)).toBeInTheDocument();
  });

  it('renders phone input', () => {
    renderWithProviders(<ComplexForm />);
    expect(screen.getByLabelText(/tel.fono/i)).toBeInTheDocument();
  });

  it('renders email input', () => {
    renderWithProviders(<ComplexForm />);
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
  });

  it('renders create button for new complex', () => {
    renderWithProviders(<ComplexForm />);
    expect(screen.getByRole('button', { name: /crear complejo/i })).toBeInTheDocument();
  });

  it('renders address input', () => {
    renderWithProviders(<ComplexForm />);
    expect(screen.getByTestId('address-input')).toBeInTheDocument();
  });
});

/**
 * The settings E2E (`e2e/specs/settings.spec.ts`) finds every field by its
 * visible label, the way someone on a screen reader does. FORM-07 wired `id`
 * onto `FormField`'s child automatically and broke exactly the fields whose
 * child is a positioning `<div>` around the control: the `<div>` took the id,
 * the real `<input>` already carried the same one, and a `for` that resolves
 * first to a non-labelable element leaves the label labelling nothing.
 *
 * Asserted as "the labelled node is the control" and "no id is claimed twice"
 * rather than through `getByLabelText` alone — testing-library resolves a
 * duplicated id more forgivingly than a browser does, which is how this
 * reached CI green and failed in Playwright.
 */
describe('ComplexForm labels the controls themselves (FORM-07 regression)', () => {
  it.each([
    ['deposit_percentage', /^porcentaje de seña$/i],
    ['cancellation_hours', /horas de cancelaci.n/i],
  ])('labels #%s, and nothing else claims that id', (id, label) => {
    renderWithProviders(<ComplexForm />);

    expect(document.querySelectorAll(`#${id}`)).toHaveLength(1);
    const control = screen.getByLabelText(label);
    expect(control.tagName).toBe('INPUT');
    expect(control.id).toBe(id);
  });

  it('gives no id to two elements at once anywhere in the form', () => {
    const { container } = renderWithProviders(<ComplexForm />);

    const ids = [...container.querySelectorAll('[id]')].map((el) => el.id);
    expect(ids).toHaveLength(new Set(ids).size);
  });
});
