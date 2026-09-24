let messages = null;

function loadMessages() {
  if (messages) {
    return messages;
  }
  const el = document.getElementById("i18n-messages");
  try {
    messages = el ? JSON.parse(el.textContent) : {};
  } catch {
    messages = {};
  }
  return messages;
}

// t looks up key among the translations base.html embeds for the current
// page's language. It falls back to the key itself if the key is missing,
// which only happens if a caller forgets to add it to base.html's list.
export function t(key) {
  const value = loadMessages()[key];
  return value === undefined ? key : value;
}
