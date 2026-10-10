/**
 * The drifting mesh the auth screens are built on, pulled out so the whole
 * app sits on the same ground instead of flat black.
 *
 * Fixed and inert: it never takes a click, never scrolls with the content,
 * and its blobs pause with the `.tab-hidden` rule when the tab is in the
 * background, so an idle tab costs nothing.
 */
export function MeshBackdrop() {
  return (
    <div className="pointer-events-none fixed inset-0 overflow-hidden" aria-hidden="true">
      <div className="auth-blob auth-blob-1 top-0 left-1/2 h-[min(55vw,600px)] w-[min(55vw,600px)] -translate-x-1/2 -translate-y-[15%]" />
      <div className="auth-blob auth-blob-2 right-0 bottom-0 h-[min(45vw,500px)] w-[min(45vw,500px)] translate-x-1/4 translate-y-1/4" />
      {/* Idle compositor/battery cost, not Speed Index, is the concern on phones — each
          blob is already near viewport size there, so 6 overlapping layers add little to
          the look. Below `sm` we drop to one blob per drift keyframe (1/2/3), keeping all
          three motions while thinning the layer count; the kept trio (top, bottom-right,
          bottom-left) still reads as a spread composition instead of stacking in one corner. */}
      <div className="auth-blob auth-blob-3 top-[40%] left-1/2 hidden h-[min(35vw,400px)] w-[min(35vw,400px)] sm:block" />
      <div className="auth-blob auth-blob-2 top-[15%] left-0 hidden h-[min(40vw,460px)] w-[min(40vw,460px)] -translate-x-1/3 sm:block" />
      <div className="auth-blob auth-blob-3 bottom-0 left-[20%] h-[min(38vw,440px)] w-[min(38vw,440px)] translate-y-1/3" />
      <div className="auth-blob auth-blob-1 top-[55%] right-[8%] hidden h-[min(36vw,420px)] w-[min(36vw,420px)] sm:block" />
      <div className="auth-noise absolute inset-0" />
    </div>
  );
}
