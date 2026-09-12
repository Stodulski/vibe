import { render, screen } from '@testing-library/react';
import { FormField } from './FormField';

describe('FormField', () => {
  it('associates the label with the input via htmlFor/id', () => {
    render(
      <FormField label="Nombre" htmlFor="name">
        <input id="name" />
      </FormField>,
    );

    expect(screen.getByLabelText('Nombre')).toBeInTheDocument();
  });

  it('gives the error message an id derived from htmlFor for aria-describedby', () => {
    render(
      <FormField label="Nombre" htmlFor="name" error="Campo requerido">
        <input id="name" aria-describedby="name-error" />
      </FormField>,
    );

    const alert = screen.getByRole('alert');
    expect(alert).toHaveAttribute('id', 'name-error');
  });
});

// FORM-07: the label and the error message were wired here, but the control's
// own half of that wiring was left to each caller — and 14 of the 33 never
// wrote it, so an error was drawn in red and announced to nobody.
describe('FormField wiring its control (FORM-07)', () => {
  it('marks the control invalid and points it at the error message', () => {
    render(
      <FormField label="Email" htmlFor="email" error="Email inválido">
        <input />
      </FormField>,
    );

    const input = screen.getByLabelText('Email');
    expect(input).toHaveAttribute('id', 'email');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAttribute('aria-describedby', 'email-error');
    expect(screen.getByRole('alert')).toHaveAttribute('id', 'email-error');
  });

  it('leaves a valid control unmarked and undescribed', () => {
    render(
      <FormField label="Email" htmlFor="email">
        <input />
      </FormField>,
    );

    const input = screen.getByLabelText('Email');
    expect(input).not.toHaveAttribute('aria-invalid');
    expect(input).not.toHaveAttribute('aria-describedby');
  });

  it('does not overwrite a control that states its own describedby', () => {
    render(
      <FormField label="Email" htmlFor="email" error="Email inválido">
        <input aria-describedby="email-help" />
      </FormField>,
    );

    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-describedby', 'email-help');
  });

  // The FORM-07 regression that broke the settings E2E: several fields wrap
  // their input in a positioning `<div>`. Wiring the wrapper gave its id away
  // twice, and `<label for>` resolves to the first element with that id — a
  // `<div>`, which is not labelable, so the field lost its label entirely.
  it('wires the control inside a positioning wrapper, not the wrapper', () => {
    const { container } = render(
      <FormField label="Porcentaje" htmlFor="pct" error="Requerido">
        <div className="relative">
          <span aria-hidden="true">%</span>
          <input id="pct" aria-invalid aria-describedby="pct-error" />
        </div>
      </FormField>,
    );

    expect(container.querySelectorAll('#pct')).toHaveLength(1);
    const control = screen.getByLabelText('Porcentaje');
    expect(control.tagName).toBe('INPUT');
    expect(container.querySelector('div.relative')).not.toHaveAttribute('id');
  });

  it('renders several children untouched rather than guessing which is the control', () => {
    render(
      <FormField label="Teléfono" htmlFor="phone" error="Requerido">
        <span>+54</span>
        <input id="phone" />
      </FormField>,
    );

    expect(screen.getByLabelText('Teléfono')).not.toHaveAttribute('aria-invalid');
    expect(screen.getByText('+54')).toBeInTheDocument();
  });
});
