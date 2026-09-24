"use strict";

function postSetup(path, payload) {
  return fetch(path, {
    method: "POST",
    credentials: "include",
    body: JSON.stringify(payload),
  }).then((response) => {
    if (!response.ok) {
      return response.text().then((error) => {
        return Promise.reject(error);
      });
    }
    return Promise.resolve();
  });
}

export async function testConnection(settings) {
  return postSetup("/api/setup/test-connection", settings);
}

export async function saveSetup(settings) {
  return postSetup("/api/setup", settings);
}
