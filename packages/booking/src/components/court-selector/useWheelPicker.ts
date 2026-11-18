import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';

/** Below this many pixels a press is a tap, not a drag. */
const TAP_SLOP = 5;
/** How much speed a fling keeps every 16ms. */
const FRICTION = 0.94;
/** How far past the first or last hour a drag may pull, in ticks, before it settles back. */
const OVERSCROLL = 0.35;

interface Motion {
  posRef: RefObject<number>;
  setPos: (value: number) => void;
  setTarget: (value: number | null) => void;
  stop: () => void;
  goTo: (index: number) => void;
  fling: (velocity: number) => void;
  /** The hour index nearest `value`, within the ruler. */
  nearest: (value: number) => number;
}

/** The wheel's position and its two animations: easing to a tick, and a fling that slows down and settles. */
function useWheelMotion(initial: number, max: number): Motion & { pos: number; target: number | null } {
  const [pos, setPosState] = useState(initial);
  const [target, setTarget] = useState<number | null>(null);
  const posRef = useRef(initial);
  const frame = useRef<number | null>(null);
  // The ruler ends at its first and last hour: a drag can pull a little past them, no further.
  const setPos = useCallback(
    (value: number) => {
      const bounded = Math.min(max + OVERSCROLL, Math.max(-OVERSCROLL, value));
      posRef.current = bounded;
      setPosState(bounded);
    },
    [max],
  );
  const nearest = useCallback((value: number) => Math.min(max, Math.max(0, Math.round(value))), [max]);
  const stop = useCallback(() => {
    if (frame.current !== null) cancelAnimationFrame(frame.current);
    frame.current = null;
  }, []);
  const goTo = useCallback(
    (index: number) => {
      stop();
      setTarget(index);
      const step = () => {
        const next = posRef.current + (index - posRef.current) * 0.22;
        const done = Math.abs(index - next) < 0.002;
        setPos(done ? index : next);
        if (done) setTarget(null);
        frame.current = done ? null : requestAnimationFrame(step);
      };
      frame.current = requestAnimationFrame(step);
    },
    [setPos, stop],
  );
  const fling = useCallback(
    (velocity: number) => {
      stop();
      let speed = velocity;
      let last = performance.now();
      const step = (now: number) => {
        const dt = Math.min(48, now - last);
        last = now;
        speed *= FRICTION ** (dt / 16);
        const atEnd = posRef.current <= -OVERSCROLL || posRef.current >= max + OVERSCROLL;
        if (Math.abs(speed) < 0.002 || atEnd) {
          goTo(nearest(posRef.current));
          return;
        }
        setPos(posRef.current + speed * dt);
        frame.current = requestAnimationFrame(step);
      };
      frame.current = requestAnimationFrame(step);
    },
    [goTo, max, nearest, setPos, stop],
  );
  useEffect(() => stop, [stop]);
  return { pos, target, posRef, setPos, setTarget, stop, goTo, fling, nearest };
}

interface Drag {
  x: number;
  pos: number;
  moved: boolean;
  lastX: number;
  lastT: number;
  velocity: number;
}

/** Dragging the wheel with a finger or a mouse; letting go flings it. A drag's closing click is not a tap. */
function usePointerDrag(motion: Motion, tickWidth: number, stepped: boolean) {
  const drag = useRef<Drag | null>(null);
  const swallowClick = useRef(false);
  return {
    onPointerDown: (event: React.PointerEvent<HTMLElement>) => {
      if (event.pointerType === 'mouse' && event.button !== 0) return;
      motion.stop();
      motion.setTarget(null);
      drag.current = {
        x: event.clientX,
        pos: motion.posRef.current,
        moved: false,
        lastX: event.clientX,
        lastT: event.timeStamp,
        velocity: 0,
      };
    },
    onPointerMove: (event: React.PointerEvent<HTMLElement>) => {
      const state = drag.current;
      if (!state) return;
      const dx = event.clientX - state.x;
      if (!state.moved && Math.abs(dx) < TAP_SLOP) return;
      if (!state.moved) {
        state.moved = true;
        event.currentTarget.setPointerCapture(event.pointerId);
      }
      const dt = Math.max(1, event.timeStamp - state.lastT);
      state.velocity = -(event.clientX - state.lastX) / tickWidth / dt;
      state.lastX = event.clientX;
      state.lastT = event.timeStamp;
      motion.setPos(state.pos - dx / tickWidth);
    },
    onPointerUp: () => {
      const state = drag.current;
      drag.current = null;
      if (!state?.moved) return;
      swallowClick.current = true;
      if (!stepped) {
        motion.fling(state.velocity);
        return;
      }
      // A few large cards: one swipe moves at most one card, toward the swipe.
      const from = Math.round(state.pos);
      const drift = motion.posRef.current - state.pos + state.velocity * 120;
      const step = Math.abs(drift) < 0.25 ? 0 : Math.sign(drift);
      motion.goTo(motion.nearest(from + step));
    },
    onPointerCancel: () => {
      drag.current = null;
      motion.goTo(motion.nearest(motion.posRef.current));
    },
    onClickCapture: (event: React.MouseEvent) => {
      if (!swallowClick.current) return;
      swallowClick.current = false;
      event.stopPropagation();
      event.preventDefault();
    },
  };
}

/** Mouse wheel and trackpad: move by their delta, then settle once they stop. */
function useWheelInput(motion: Motion, tickWidth: number) {
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(
    () => () => {
      clearTimeout(timer.current);
    },
    [],
  );
  const { stop, setTarget, setPos, posRef, goTo, nearest } = motion;
  return useCallback(
    (event: WheelEvent) => {
      const delta = Math.abs(event.deltaY) > Math.abs(event.deltaX) ? event.deltaY : event.deltaX;
      if (delta === 0) return;
      event.preventDefault();
      stop();
      setTarget(null);
      setPos(posRef.current + delta / tickWidth);
      clearTimeout(timer.current);
      timer.current = setTimeout(() => {
        goTo(nearest(posRef.current));
      }, 120);
    },
    [goTo, nearest, posRef, setPos, setTarget, stop, tickWidth],
  );
}

/**
 * The position of the wheel, in ticks from its first item to its last; `stepped`
 * makes each swipe move at most one item (a few large cards) instead of flinging.
 * The position is moved by dragging with a fling that settles
 * on the nearest tick, by the mouse wheel, and by `goTo`. Position and drawing come from
 * the same state in the same frame, so the wheel never lags behind the finger. `center`
 * is the tick at the center, or the one a settle or a tap is heading to.
 */
export function useWheelPicker(initial: number, count: number, tickWidth: number, options: { stepped?: boolean } = {}) {
  const motion = useWheelMotion(initial, Math.max(0, count - 1));
  const handlers = usePointerDrag(motion, tickWidth, options.stepped ?? false);
  const onWheel = useWheelInput(motion, tickWidth);
  return { pos: motion.pos, center: motion.target ?? Math.round(motion.pos), handlers, onWheel, goTo: motion.goTo };
}
