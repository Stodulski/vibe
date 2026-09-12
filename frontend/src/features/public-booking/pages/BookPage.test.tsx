import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import BookPage from './BookPage';

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/:slug/book" element={<BookPage />} />
        <Route path="/:slug" element={<div>complex page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('BookPage', () => {
  it('redirects to the complex page', () => {
    renderAt('/club-norte/book');
    expect(screen.getByText('complex page')).toBeInTheDocument();
  });

  it('forwards the error query param to the complex page redirect', () => {
    renderAt('/club-norte/book?error=slot_taken');
    expect(screen.getByText('complex page')).toBeInTheDocument();
  });
});
