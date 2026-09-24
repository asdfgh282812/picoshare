export function setLanguage(language) {
  return fetch("/api/language", {
    method: "PUT",
    credentials: "include",
    body: JSON.stringify({ language }),
  }).then((response) => {
    if (!response.ok) {
      return response.text().then((error) => {
        return Promise.reject(error);
      });
    }
    return Promise.resolve();
  });
}
