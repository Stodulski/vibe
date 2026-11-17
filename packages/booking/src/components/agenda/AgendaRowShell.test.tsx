import { describe, expect, it, vi } from 'vitest';
import { screen, within } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { AgendaRowShell } from './AgendaRowShell';

/** A row for Wednesday 18 November 2026, the day the owner checked on a 320px phone. */
function renderRow() {
  return renderBooking(
    <AgendaRowShell
      date="2026-11-18"
      label="Cancha abierta de pádel"
      title="Abierta a todos"
      meta={[
        { key: 'time', text: '19:00 a 22:00' },
        { key: 'spots', text: 'Quedan 7 lugares' },
      ]}
      viewDetailLabel="Ver detalle"
      onViewDetail={vi.fn()}
      signUp={{ href: '/c/test-club/events/ev1', label: 'Inscribirme' }}
    />,
  );
}

/** The row's article and its date block, the `time` element. */
function dateBlockOf() {
  const article = screen.getByRole('article');
  const time = article.querySelector('time');
  if (!time) throw new Error('no date block');
  return { article, time };
}

describe('AgendaRowShell date block', () => {
  it('writes the date before the detail, so a phone stacks it above the label, title and meta', () => {
    renderRow();
    const { time } = dateBlockOf();
    const heading = screen.getByRole('heading', { name: 'Abierta a todos' });
    const label = screen.getByText('Cancha abierta de pádel');

    expect(time.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(time.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('reads as one line on a phone: the weekday, the day and the month', () => {
    renderRow();
    const { time } = dateBlockOf();

    expect(Array.from(time.children).map((part) => part.textContent)).toEqual(['Mié', '18', 'nov']);
    expect(time).toHaveClass('flex-row', 'sm:flex-col');
  });

  it('shows the day number in green', () => {
    renderRow();

    expect(screen.getByText('18')).toHaveClass('text-primary-400');
  });

  it('has the accent rule only from sm up', () => {
    renderRow();
    const { time } = dateBlockOf();
    const rule = time.nextElementSibling;

    expect(rule).toHaveAttribute('aria-hidden', 'true');
    expect(rule).toHaveClass('hidden', 'sm:block');
  });

  it('stacks the row on a phone and sets it side by side from sm up, with a full-width sign-up', () => {
    renderRow();
    const { article } = dateBlockOf();
    const signUp = within(article).getByRole('link', { name: /Inscribirme/ });

    expect(article).toHaveClass('flex-col', 'sm:flex-row');
    expect(signUp).toHaveClass('w-full', 'sm:w-auto');
  });
});
