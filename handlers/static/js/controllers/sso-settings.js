"use strict";

export async function getOidcSettings() {
  return fetch("/api/admin/oidc-settings", {
    method: "GET",
    credentials: "include",
  }).then((response) => {
    if (!response.ok) {
      return response.text().then((error) => {
        return Promise.reject(error);
      });
    }
    return response.json();
  });
}

function putOidcSettings(path, settings) {
  return fetch(path, {
    method: "PUT",
    credentials: "include",
    body: JSON.stringify(settings),
  }).then((response) => {
    if (!response.ok) {
      return response.text().then((error) => {
        return Promise.reject(error);
      });
    }
    return Promise.resolve();
  });
}

function postOidcSettings(path, settings) {
  return fetch(path, {
    method: "POST",
    credentials: "include",
    body: JSON.stringify(settings),
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
  return postOidcSettings("/api/admin/oidc-settings/test-connection", settings);
}

export async function saveOidcSettings(settings) {
  return putOidcSettings("/api/admin/oidc-settings", settings);
}
