const storageKey = "ps_theme";

function currentTheme() {
  return document.documentElement.getAttribute("data-bs-theme") === "dark"
    ? "dark"
    : "light";
}

function updateToggleIcon(theme) {
  const icon = document.getElementById("theme-toggle-icon");
  if (!icon) {
    return;
  }
  icon.classList.toggle("fa-sun", theme === "dark");
  icon.classList.toggle("fa-moon", theme !== "dark");
}

function setTheme(theme) {
  document.documentElement.setAttribute("data-bs-theme", theme);
  try {
    localStorage.setItem(storageKey, theme);
  } catch {
    // Ignore storage errors (e.g., private browsing).
  }
  updateToggleIcon(theme);
}

export function initThemeToggle() {
  updateToggleIcon(currentTheme());
  const toggle = document.getElementById("theme-toggle");
  if (!toggle) {
    return;
  }
  toggle.addEventListener("click", () => {
    setTheme(currentTheme() === "dark" ? "light" : "dark");
  });
}
