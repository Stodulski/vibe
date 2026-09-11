import { memo, useContext, useEffect, useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PageHeadingProvider } from './PageHeadingProvider';
import { PageHeadingContext } from './pageHeadingContextObject';

let consumerRenderCount = 0;

// `memo` isolates the effect under test: without it, `Consumer` would
// re-render whenever its parent (`PageHeadingProvider`) re-renders for ANY
// reason, masking whether the context `value` itself changed identity.
const Consumer = memo(function Consumer() {
  // Counted in an effect (a sanctioned side-effect location), not the render
  // body itself, to keep this test component's render pure.
  useEffect(() => {
    consumerRenderCount += 1;
  });
  const ctx = useContext(PageHeadingContext);
  return <div>{ctx?.heading?.title ?? 'none'}</div>;
});

function Harness() {
  const [tick, setTick] = useState(0);
  return (
    <PageHeadingProvider>
      {/* Unrelated ancestor state — re-renders PageHeadingProvider without
          ever calling its own `setHeading`. */}
      <button
        type="button"
        onClick={() => {
          setTick((t) => t + 1);
        }}
      >
        tick ({tick})
      </button>
      <Consumer />
    </PageHeadingProvider>
  );
}

describe('PageHeadingProvider (B12)', () => {
  it('does not re-render context consumers when only an unrelated ancestor re-renders', async () => {
    consumerRenderCount = 0;
    const user = userEvent.setup();
    render(<Harness />);
    expect(consumerRenderCount).toBe(1);

    await user.click(screen.getByRole('button', { name: /tick/ }));

    expect(consumerRenderCount).toBe(1);
  });
});
