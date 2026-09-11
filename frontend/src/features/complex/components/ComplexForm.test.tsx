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
