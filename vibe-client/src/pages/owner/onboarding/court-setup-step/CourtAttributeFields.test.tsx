import { render, screen } from '@testing-library/react';
import { CourtAttributeFields } from './CourtAttributeFields';

describe('CourtAttributeFields', () => {
  it('associates the sport and court type selects with their labels', () => {
    render(<CourtAttributeFields setValue={vi.fn()} />);
    // `getByLabelText` only finds an element whose `<label htmlFor>` (or
    // aria-labelledby) actually resolves to it — before this fix, neither
    // select had an `id`, so both labels pointed at nothing.
    expect(screen.getByLabelText('Deporte')).toBeInTheDocument();
    expect(screen.getByLabelText('Tipo de cancha')).toBeInTheDocument();
  });
});
