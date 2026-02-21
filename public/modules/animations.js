function hasGSAP() {
  return typeof window !== "undefined" && Boolean(window.gsap);
}

export function prefersReducedMotion() {
  return Boolean(
    typeof window !== "undefined" &&
      window.matchMedia &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

function makeVisible(el) {
  if (!el) return;
  el.style.opacity = "1";
  el.style.transform = "none";
}

export function fadeIn(el, opts = {}) {
  if (!el) return null;
  if (prefersReducedMotion() || !hasGSAP()) {
    makeVisible(el);
    return null;
  }
  return window.gsap.fromTo(
    el,
    { opacity: 0, y: 10 },
    { opacity: 1, y: 0, duration: 0.3, ease: "power2.out", ...opts }
  );
}

export function slideDown(el, opts = {}) {
  if (!el) return null;
  if (prefersReducedMotion() || !hasGSAP()) {
    makeVisible(el);
    return null;
  }
  return window.gsap.fromTo(
    el,
    { opacity: 0, y: -20 },
    { opacity: 1, y: 0, duration: 0.45, ease: "power2.out", ...opts }
  );
}

export function stagger(els, opts = {}) {
  const items = Array.from(els || []).filter(Boolean);
  if (!items.length) return null;
  if (prefersReducedMotion() || !hasGSAP()) {
    items.forEach(makeVisible);
    return null;
  }
  return window.gsap.fromTo(
    items,
    { opacity: 0, y: 16 },
    { opacity: 1, y: 0, duration: 0.4, stagger: 0.08, ease: "power2.out", ...opts }
  );
}
