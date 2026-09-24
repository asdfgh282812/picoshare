"use strict";

export async function grantAdmin(id) {
  return fetch(`/api/admin/users/${id}/grant-admin`, {
    method: "PUT",
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

export async function revokeAdmin(id) {
  return fetch(`/api/admin/users/${id}/revoke-admin`, {
    method: "PUT",
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
