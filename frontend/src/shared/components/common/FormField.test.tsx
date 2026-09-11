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
