export function createA11yAnnouncer(options = {}) {
  const { region } = options;

  function announce(message) {
    if (!region) return;
    const text = String(message || "").trim();
    if (!text) return;
    region.textContent = "";
    requestAnimationFrame(() => {
      region.textContent = text;
    });
  }

  return {
    announce
  };
}
