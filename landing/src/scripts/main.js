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

/* The homepage's bar starts with its account button as an outline (nav--quiet), because
   the hero right under it already carries the filled one. Once that one has scrolled out
   of view the bar's becomes the filled button, so each screen has exactly one. */
const heroCta = document.querySelector('.hero-ctas .btn-primary');
if (heroCta && nav.classList.contains('nav--quiet')) {
  new IntersectionObserver((entries) => {
    entries.forEach(e => nav.classList.toggle('nav--quiet', e.isIntersecting));
  }).observe(heroCta);
}

/* Hide the WhatsApp button while the hero, the FAQ or the footer is on screen: the
   hero has its own calls to action, the FAQ links to WhatsApp itself, and in both the
   FAQ and the footer the button would sit over controls in the same right-hand column. */
const fab = document.querySelector('.whatsapp-fab');
const fabBlockers = ['.hero', '.faq', '.footer'].map(s => document.querySelector(s)).filter(Boolean);
if (fab && fabBlockers.length) {
  const onScreen = new Set();
  const fabObserver = new IntersectionObserver((entries) => {
    entries.forEach(e => (e.isIntersecting ? onScreen.add(e.target) : onScreen.delete(e.target)));
    fab.classList.toggle('fab-hidden', onScreen.size > 0);
  }, { threshold: 0.15 });
  fabBlockers.forEach(el => fabObserver.observe(el));
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
if (blob && window.matchMedia('(pointer: fine)').matches) {
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

/* Modules — one of the eight shows at a time. Every module is in the markup and CSS
   shows only the one marked .active, so this only moves that mark: a named button picks a
   module, the arrows step through them and wrap around at either end. */
const ftBlocks = [...document.querySelectorAll('.ft-block')];
const ftTabs = [...document.querySelectorAll('.ft-tab')];
const ftTabStrip = document.querySelector('.ft-tabs');
const ftPrev = document.querySelector('.ft-arrow-prev');
const ftNext = document.querySelector('.ft-arrow-next');
let ftIndex = 0;

function setCurrent(el, isCurrent) {
  el.classList.toggle('active', isCurrent);
  if (isCurrent) el.setAttribute('aria-current', 'true');
  else el.removeAttribute('aria-current');
}

function goToFeature(index) {
  ftIndex = (index + ftBlocks.length) % ftBlocks.length;
  ftBlocks.forEach((block, i) => setCurrent(block, i === ftIndex));
  ftTabs.forEach((tab, i) => setCurrent(tab, i === ftIndex));
  /* On a phone the row of names scrolls sideways: keep the chosen one in sight. Moving
     the row's own scroll position, not scrollIntoView, which could also move the page. */
  const tab = ftTabs[ftIndex];
  if (tab && ftTabStrip && ftTabStrip.scrollWidth > ftTabStrip.clientWidth) {
    ftTabStrip.scrollTo({
      left: tab.offsetLeft - (ftTabStrip.clientWidth - tab.offsetWidth) / 2,
      behavior: reduceMotion ? 'auto' : 'smooth',
    });
  }
}

if (ftBlocks.length) {
  if (ftPrev) ftPrev.addEventListener('click', () => goToFeature(ftIndex - 1));
  if (ftNext) ftNext.addEventListener('click', () => goToFeature(ftIndex + 1));
  ftTabs.forEach((tab, i) => tab.addEventListener('click', () => goToFeature(i)));
}
