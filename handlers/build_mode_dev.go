//go:build dev

package handlers

// Development builds must always serve the latest frontend source files.
const staticCacheControl = "no-store"

// devLoginEnabled lets development and e2e-test builds log in as any user
// without a real identity provider.
const devLoginEnabled = true
