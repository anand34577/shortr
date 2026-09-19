// Runs before React mounts to avoid a flash of the wrong theme. Kept as an
// external file (not inline) so it can run under a strict CSP with no
// 'unsafe-inline' script-src.
(function () {
  try {
    var stored = localStorage.getItem('shortr-theme');
    var theme = stored === 'light' || stored === 'dark'
      ? stored
      : (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.documentElement.style.colorScheme = theme;
  } catch {}
})();
