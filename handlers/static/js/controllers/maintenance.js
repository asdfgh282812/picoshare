"use strict";

export async function cleanUpDatabase() {
  return fetch("/api/maintenance/cleanup", {
    method: "POST",
    credentials: "include",
  })
    .then((response) => {
      if (!response.ok) {
        return response.text().then((error) => {
          return Promise.reject(error);
        });
      }
      return Promise.resolve();
    })
    .catch((error) => {
      if (error.message) {
        return Promise.reject(
          "Failed to communicate with server" +
            (error.message ? `: ${error.message}` : "."),
        );
      }
      return Promise.reject(error);
    });
}
