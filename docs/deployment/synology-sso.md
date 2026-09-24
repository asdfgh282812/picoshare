# Single sign-on setup

PicoShare authenticates through OpenID Connect (OIDC) single sign-on instead of a shared password. This guide uses Synology SSO Server, but any standards-compliant OIDC identity provider works the same way.

## Register PicoShare as an application

1. On your Synology NAS, open **SSO Server** and go to **Applications**.
2. Create a new application (an OIDC client) for PicoShare.
3. Set its callback URL to `https://<your-picoshare-host>/oidc/callback`. This must match exactly, including the scheme (`https://`).
4. Note the **Client ID** and **Client Secret** SSO Server generates. You'll need both during setup.

## Find your one-time setup token

The first time PicoShare starts without a configured identity provider, it prints a one-time setup token to its log:

```
=================================================================
PicoShare has no identity provider configured yet.
Visit /setup and enter this one-time setup token: <token>
=================================================================
```

Find this line with `docker logs picoshare` (or your platform's equivalent), or by watching PicoShare's startup output directly.

## Complete setup

1. Visit `https://<your-picoshare-host>/setup`.
2. Enter the setup token from the log.
3. Enter your identity provider's **Issuer URL**. For Synology SSO Server, this is `https://<your-nas-host>/webman/sso`.
4. Enter the **Client ID** and **Client Secret** from the application you registered.
5. Confirm the **Redirect URL** matches what you registered in SSO Server.
6. Select **Test Connection** to confirm PicoShare can reach your identity provider before saving.
7. Select **Save and Log In**. PicoShare redirects you to your identity provider to complete the first login.

**The first person to log in becomes PicoShare's administrator.** If you're upgrading an existing PicoShare instance, that first login also claims every file and guest link that existed before multi-user support, so log in immediately after completing setup.

An administrator can see and manage every user's files, and can change PicoShare's settings. Everyone else only sees their own files.

## If a reverse proxy makes your issuer URL unreachable

Some reverse proxy setups make the issuer URL unreachable under its own name from inside PicoShare's container. If discovery fails with an issuer mismatch error, set the **Expected issuer** field on the setup page to the issuer value your identity provider actually reports (visible at `<issuer-url>/.well-known/openid-configuration`).

## Self-signed certificates

If your identity provider uses a certificate that isn't signed by a public certificate authority, paste its CA certificate (PEM format) into the **CA certificate** field during setup.

## Recovering from a misconfiguration

If you configure single sign-on incorrectly and can no longer log in, run PicoShare with `-reset-oidc`:

```bash
docker exec picoshare /app/picoshare -db /data/store.db -reset-oidc
```

This clears the identity provider configuration and prints a new setup token. Restart PicoShare normally afterward and visit `/setup` again.

## Limitations

- Logging out of PicoShare doesn't log you out of your identity provider. Clicking "Log In" again signs you back in without a prompt.
- Anyone who can authenticate with your identity provider can create a PicoShare account. Restrict access in your identity provider's own application settings (for example, in SSO Server's application configuration) if you need to limit who can reach PicoShare.
