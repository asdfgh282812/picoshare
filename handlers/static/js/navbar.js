import { logOut } from "./controllers/auth.js";
import { setLanguage } from "./controllers/language.js";
import { initThemeToggle } from "./controllers/theme.js";

initThemeToggle();

const logOutEl = document.getElementById("navbar-log-out");
if (logOutEl) {
  logOutEl.addEventListener("click", () => {
    logOut().then(() => {
      document.location = "/";
    });
  });
}

document.querySelectorAll(".navbar-language-option").forEach((btn) => {
  btn.addEventListener("click", () => {
    setLanguage(btn.getAttribute("data-lang")).then(() => {
      window.location.reload();
    });
  });
});
