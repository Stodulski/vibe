/* Scroll animations */
const obs = new IntersectionObserver((entries) => {
  entries.forEach(e => { if (e.isIntersecting) e.target.classList.add('visible'); });
}, { threshold: 0.08, rootMargin: '0px 0px -40px 0px' });
document.querySelectorAll('.anim').forEach(el => obs.observe(el));

/* Nav scroll */
const nav = document.querySelector('.nav');
window.addEventListener('scroll', () => {
  nav.classList.toggle('scrolled', window.scrollY > 32);
}, { passive: true });

/* Hide WhatsApp FAB while it would overlap the hero content */
const hero = document.querySelector('.hero');
const fab = document.querySelector('.whatsapp-fab');
if (hero && fab) {
  const heroObserver = new IntersectionObserver((entries) => {
    entries.forEach(e => fab.classList.toggle('fab-hidden', e.isIntersecting));
  }, { threshold: 0.15 });
  heroObserver.observe(hero);
}

/* Mobile menu */
const toggle = document.querySelector('.nav-toggle');
const mobileMenu = document.querySelector('.mobile-menu');
toggle.addEventListener('click', () => {
  const isOpen = mobileMenu.classList.toggle('open');
  toggle.classList.toggle('active');
  toggle.setAttribute('aria-expanded', isOpen);
  mobileMenu.setAttribute('aria-hidden', !isOpen);
  document.body.style.overflow = isOpen ? 'hidden' : '';
});
mobileMenu.querySelectorAll('a').forEach(link => {
  link.addEventListener('click', () => {
    mobileMenu.classList.remove('open');
    toggle.classList.remove('active');
    toggle.setAttribute('aria-expanded', 'false');
    mobileMenu.setAttribute('aria-hidden', 'true');
    document.body.style.overflow = '';
  });
});

/* Cursor blob — throttled with rAF */
const blob = document.querySelector('.cursor-blob');
if (window.matchMedia('(pointer: fine)').matches) {
  let rafId = 0;
  document.addEventListener('mousemove', (e) => {
    cancelAnimationFrame(rafId);
    rafId = requestAnimationFrame(() => {
      /* translate(-50%, -50%) is applied first and re-centres the blob on the
         point, so the pointer coordinates can be used as-is. */
      blob.style.transform =
        `translate3d(${e.clientX}px, ${e.clientY}px, 0) translate(-50%, -50%)`;
    });
  });
}

/* Active nav link based on scroll position */
const sections = document.querySelectorAll('section[id]');
const navAnchors = document.querySelectorAll('.nav-links a[href^="#"]');
const navObserver = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      navAnchors.forEach(a => a.classList.remove('active'));
      const active = document.querySelector('.nav-links a[href="#' + entry.target.id + '"]');
      if (active) active.classList.add('active');
    }
  });
}, { threshold: 0.3, rootMargin: '-80px 0px -50% 0px' });
sections.forEach(s => navObserver.observe(s));

/* Smooth scroll */
document.querySelectorAll('a[href^="#"]').forEach(a => {
  a.addEventListener('click', function(e) {
    const t = document.querySelector(this.getAttribute('href'));
    if (!t) return;
    e.preventDefault();
    /* Close mobile menu first if open */
    if (mobileMenu.classList.contains('open')) {
      mobileMenu.classList.remove('open');
      toggle.classList.remove('active');
      toggle.setAttribute('aria-expanded', 'false');
      mobileMenu.setAttribute('aria-hidden', 'true');
      document.body.style.overflow = '';
    }
    /* Reveal all anim elements so layout is stable before scrolling */
    document.querySelectorAll('.anim:not(.visible)').forEach(el => el.classList.add('visible'));
    requestAnimationFrame(() => {
      const navH = window.innerWidth <= 480 ? 56 : 72;
      window.scrollTo({ top: t.getBoundingClientRect().top + window.scrollY - navH, behavior: 'smooth' });
    });
  });
});

/* Counter animation for stats */
function animateCounter(el, target, prefix, suffix) {
  const duration = 1500;
  const start = performance.now();
  const textNode = document.createTextNode(prefix + '0');
  const suffixSpan = suffix ? document.createElement('span') : null;
  if (suffixSpan) suffixSpan.textContent = suffix;
  el.textContent = '';
  el.appendChild(textNode);
  if (suffixSpan) el.appendChild(suffixSpan);
  const step = (now) => {
    const progress = Math.min((now - start) / duration, 1);
    const eased = 1 - Math.pow(1 - progress, 3);
    const current = Math.round(eased * target);
    textNode.textContent = prefix + current;
    if (progress < 1) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

const statsObserver = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting && !entry.target.dataset.counted) {
      entry.target.dataset.counted = 'true';
      const el = entry.target;
      const text = el.textContent.trim();

      if (text === '24/7') {
        animateCounter(el, 24, '', '/7');
      } else if (text.startsWith('<') || text.startsWith('\u003c')) {
        const num = parseInt(text.replace(/[^0-9]/g, ''));
        const suffix = text.replace(/[0-9<\u003c]/g, '');
        animateCounter(el, num, '<', suffix);
      } else {
        const num = parseInt(text.replace(/[^0-9]/g, ''));
        const suffix = text.replace(/[0-9]/g, '');
        if (!isNaN(num)) {
          animateCounter(el, num, '', suffix);
        }
      }
    }
  });
}, { threshold: 0.5 });

document.querySelectorAll('.stat-value').forEach(el => statsObserver.observe(el));


/* Timeline spine fill */
const timeline = document.querySelector('.how-timeline');
if (timeline) {
  const spineFill = timeline.querySelector('.how-spine-fill');

  /* The timeline's box is measured once instead of on every scroll event: reading
     it there forced a synchronous layout, because the previous event had just
     written the fill's height and the read had to flush it first. That cost 55ms
     of forced reflow over a single pass down the page.

     Caching is safe here because nothing above the timeline shifts after load —
     every image carries width/height and measured CLS is 0. */
  let timelineTop = 0;
  let timelineHeight = 1;

  function measureSpine() {
    const rect = timeline.getBoundingClientRect();
    timelineTop = rect.top + window.scrollY;
    timelineHeight = rect.height || 1;
  }

  /* Scroll can fire several times per frame; the fill only needs one write. */
  let spineQueued = false;
  function updateSpine() {
    if (spineQueued) return;
    spineQueued = true;
    requestAnimationFrame(() => {
      spineQueued = false;
      const scrolled = window.scrollY + window.innerHeight * 0.5 - timelineTop;
      const progress = Math.max(0, Math.min(1, scrolled / timelineHeight));
      spineFill.style.height = (progress * 100) + '%';
    });
  }

  measureSpine();
  window.addEventListener('scroll', updateSpine, { passive: true });
  window.addEventListener('resize', () => { measureSpine(); updateSpine(); }, { passive: true });
  window.addEventListener('load', () => { measureSpine(); updateSpine(); }, { once: true });
  updateSpine();
}

/* FAQ accordion — animated height, driven by JS since <details> content is
   skipped from layout while closed, which defeats a pure-CSS transition. */
const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
const FAQ_DURATION = 350;
function animateFaqHeight(content, from, to, onDone) {
  if (content._faqTimer) { clearTimeout(content._faqTimer); content._faqTimer = null; }
  if (reduceMotion) {
    content.style.height = to + 'px';
    if (onDone) onDone();
    return;
  }
  content.style.height = from + 'px';
  void content.offsetHeight; /* force a reflow so the browser commits the starting height first */
  content.style.height = to + 'px';
  content._faqTimer = setTimeout(() => {
    content._faqTimer = null;
    if (onDone) onDone();
  }, FAQ_DURATION + 20);
}
function closeFaqItem(item) {
  const content = item.querySelector('p');
  /* Use the current rendered height (not scrollHeight) as the starting point —
     the open height includes FAQ_ANSWER_GAP, which scrollHeight doesn't know about. */
  const from = content.getBoundingClientRect().height;
  animateFaqHeight(content, from, 0, () => {
    item.open = false;
    content.style.height = '';
    /* <details> can leave layout stale right after closing — force a reflow
       so height:0 actually takes effect instead of "remembering" the open size. */
    void content.offsetHeight;
  });
}
const FAQ_ANSWER_GAP = 20; /* matches the old padding-bottom, now baked into the animated height */
function openFaqItem(item) {
  document.querySelectorAll('.faq-item[open]').forEach(other => {
    if (other !== item) closeFaqItem(other);
  });
  item.open = true;
  const content = item.querySelector('p');
  const target = content.scrollHeight + FAQ_ANSWER_GAP;
  /* Keep the measured height pinned while open — clearing it back to
     'auto' would fall through to the base (closed) height:0 rule. */
  animateFaqHeight(content, 0, target);
}
/* Pin the height of any FAQ item marked open in the static markup (e.g. the first one)
   so it doesn't start collapsed at the base height:0 before this script runs. */
document.querySelectorAll('.faq-item[open] p').forEach(content => {
  content.style.height = (content.scrollHeight + FAQ_ANSWER_GAP) + 'px';
});
document.querySelectorAll('.faq-item').forEach(item => {
  item.querySelector('summary').addEventListener('click', (e) => {
    e.preventDefault();
    if (item.open) closeFaqItem(item);
    else openFaqItem(item);
  });
});
/* Re-measure open answers on resize (e.g. rotation, font reflow) so the pinned height stays accurate. */
let faqResizeTimer;
window.addEventListener('resize', () => {
  clearTimeout(faqResizeTimer);
  faqResizeTimer = setTimeout(() => {
    document.querySelectorAll('.faq-item[open] p').forEach(content => {
      content.style.height = 'auto';
      const h = content.scrollHeight + FAQ_ANSWER_GAP;
      content.style.height = h + 'px';
    });
  }, 150);
});

/* Comparison mobile tabs — switch which column (Vibe / WhatsApp / ATC) is shown */
const comparisonTabs = document.querySelectorAll('.comparison-tab');
comparisonTabs.forEach(tab => {
  tab.addEventListener('click', () => {
    const target = tab.dataset.target;
    comparisonTabs.forEach(t => {
      t.classList.toggle('active', t === tab);
      t.setAttribute('aria-selected', t === tab);
    });
    /* One attribute on the table instead of a class per column: the CSS drops
       every cell that does not belong to the chosen one. */
    document.querySelector('.comparison-table').dataset.shown = target;
  });
});

/* Features carousel (desktop) — the track is moved with a JS-computed transform instead of
   native scrolling: Chromium under-reports scrollWidth for trailing space in a scrollable
   flex row, which makes the first and last slide impossible to center. Mobile stacks every
   slide full-width and neutralizes the transform in CSS. */
const ftBlocks = [...document.querySelectorAll('.ft-block:not(.ft-block--clone)')];
const ftAllBlocks = [...document.querySelectorAll('.ft-block')];
/* Wrap targets. Two more decorative clones sit further out (see Features.astro) so these
   two also have a neighbor to peek at while they're centered, hence the data-clone match. */
const ftCloneBefore = document.querySelector('.ft-block--clone[data-clone="before"]');
const ftCloneAfter = document.querySelector('.ft-block--clone[data-clone="after"]');
const ftDots = [...document.querySelectorAll('.ft-dot')];
const ftViewport = document.querySelector('.ft-viewport');
const ftStack = document.querySelector('.ft-stack');
const ftPrev = document.querySelector('.ft-arrow-prev');
const ftNext = document.querySelector('.ft-arrow-next');
const ftMobileQuery = window.matchMedia('(max-width: 768px)');
const FT_TRANSITION_MS = 520; /* the 0.5s CSS transition on .ft-stack, plus a hair */
let ftIndex = 0;
let ftSnapTimer = null;

/* Centers any slide — real or clone — in the viewport. */
function ftCenterOn(el, animate) {
  if (ftMobileQuery.matches || !el) return;
  const offset = el.offsetLeft - (ftViewport.clientWidth - el.offsetWidth) / 2;
  if (animate) {
    ftStack.style.transform = `translateX(${-offset}px)`;
    return;
  }
  ftStack.style.transition = 'none';
  ftStack.style.transform = `translateX(${-offset}px)`;
  void ftStack.offsetHeight;
  ftStack.style.transition = '';
}

/* .active styles whichever slide is centered right now, clones included — otherwise the one
   sitting in the main spot mid-wrap renders as a dimmed secondary slide. */
function ftSetActiveElement(el) {
  ftAllBlocks.forEach(block => block.classList.toggle('active', block === el));
}

function ftSetIndex(index) {
  ftIndex = index;
  ftDots.forEach((dot, i) => dot.classList.toggle('active', i === ftIndex));
}

/* Lands on the real slide with every transition suppressed: position, dim and scale all
   have to change in the same invisible frame, or the swap shows up as a jump or a fade-in. */
function ftSnapToReal(clone) {
  const real = ftBlocks[ftIndex];
  if (clone) clone.classList.add('ft-no-anim');
  real.classList.add('ft-no-anim');
  ftCenterOn(real, false);
  ftSetActiveElement(real);
  void ftStack.offsetHeight;
  if (clone) clone.classList.remove('ft-no-anim');
  real.classList.remove('ft-no-anim');
}

/* Also the resize handler: a pending snap must not fire against a stale layout. */
function updateFeatureCarousel() {
  if (ftSnapTimer) { clearTimeout(ftSnapTimer); ftSnapTimer = null; }
  ftCenterOn(ftBlocks[ftIndex], false);
  ftSetActiveElement(ftBlocks[ftIndex]);
}

/* Wrapping past either end animates onto that end's clone, then snaps to the matching real
   slide once the animation is done — same pixels, so the loop looks seamless. */
function goToFeature(index) {
  /* A click mid-wrap has to complete the pending snap first: the track is still parked out
     by the clone, so measuring the next move from there would fling it across the track. */
  if (ftSnapTimer) {
    clearTimeout(ftSnapTimer);
    ftSnapTimer = null;
    ftSnapToReal();
  }
  const wrapsBack = index < 0;
  if (!wrapsBack && index < ftBlocks.length) {
    ftSetIndex(index);
    ftSetActiveElement(ftBlocks[ftIndex]);
    ftCenterOn(ftBlocks[ftIndex], true);
    return;
  }
  ftSetIndex(wrapsBack ? ftBlocks.length - 1 : 0);
  if (reduceMotion) {
    /* No animation to hide the hop behind, so go straight to the real slide. */
    ftSetActiveElement(ftBlocks[ftIndex]);
    ftCenterOn(ftBlocks[ftIndex], false);
    return;
  }
  const clone = wrapsBack ? ftCloneBefore : ftCloneAfter;
  ftCenterOn(clone, true);
  ftSetActiveElement(clone);
  ftSnapTimer = setTimeout(() => {
    ftSnapTimer = null;
    ftSnapToReal(clone);
  }, FT_TRANSITION_MS);
}

if (ftViewport && ftStack && ftBlocks.length) {
  updateFeatureCarousel();
  if (ftPrev) ftPrev.addEventListener('click', () => goToFeature(ftIndex - 1));
  if (ftNext) ftNext.addEventListener('click', () => goToFeature(ftIndex + 1));
  ftDots.forEach((dot, i) => dot.addEventListener('click', () => goToFeature(i)));
  let ftResizeTimer;
  window.addEventListener('resize', () => {
    clearTimeout(ftResizeTimer);
    ftResizeTimer = setTimeout(updateFeatureCarousel, 150);
  });
}

/* Hero video — it carries the product story, so it plays for everyone. Visitors
   who asked for less motion still get it, but with a control to stop it. */
const heroVideo = document.querySelector('.phone-video');
if (heroVideo) {
  /* Phones refuse the autoplay attribute more often than desktops do — Low Power
     Mode and data-saver both block it, and Safari wants `muted` set as a property,
     not just an attribute. So ask for playback explicitly, and if the browser still
     says no, start on the first thing the visitor does: that counts as the user
     gesture these policies are waiting for. */
  heroVideo.muted = true;

  /* A deliberate pause has to outrank every automatic retry below, or the visitor
     presses the button and the next scroll starts the video again. */
  let heroPausedByVisitor = false;

  function playHeroVideo() {
    if (heroPausedByVisitor) return;
    const attempt = heroVideo.play();
    if (attempt) attempt.catch(() => { /* blocked — the listeners below retry */ });
  }

  const gestures = ['touchstart', 'pointerdown', 'scroll'];
  function playOnGesture() {
    playHeroVideo();
    gestures.forEach(e => window.removeEventListener(e, playOnGesture));
  }
  gestures.forEach(e => window.addEventListener(e, playOnGesture, { once: true, passive: true }));

  playHeroVideo();
  heroVideo.addEventListener('canplay', playHeroVideo, { once: true });

  /* Some mobile browsers drop playback once the element scrolls away and never
     resume it, leaving a frozen frame when the visitor scrolls back up. */
  new IntersectionObserver((entries) => {
    entries.forEach(entry => { if (entry.isIntersecting && heroVideo.paused) playHeroVideo(); });
  }, { threshold: 0.25 }).observe(heroVideo);

  /* Built in JS so the button only exists for the visitors it serves — everyone
     else keeps the mockup clean. */
  if (reduceMotion) {
    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'phone-video-toggle';

    function paintToggle() {
      const paused = heroVideo.paused;
      toggle.textContent = paused ? 'Reproducir' : 'Pausar';
      toggle.setAttribute('aria-label', paused
        ? 'Reproducir el video de demostración'
        : 'Pausar el video de demostración');
    }

    toggle.addEventListener('click', () => {
      if (heroVideo.paused) {
        heroPausedByVisitor = false;
        playHeroVideo();
      } else {
        heroPausedByVisitor = true;
        heroVideo.pause();
      }
    });

    heroVideo.addEventListener('play', paintToggle);
    heroVideo.addEventListener('pause', paintToggle);
    paintToggle();
    heroVideo.closest('.phone-frame').appendChild(toggle);
  }
}

