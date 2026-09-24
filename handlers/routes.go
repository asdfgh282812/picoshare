package handlers

import "net/http"

func (s *Server) routes() {
	s.router.Use(s.loadSession)

	s.router.HandleFunc("/api/auth", s.authDelete()).Methods(http.MethodDelete)
	s.router.HandleFunc("/api/language", s.languagePut()).Methods(http.MethodPut)
	s.router.HandleFunc("/oidc/login", s.oidcLoginGet()).Methods(http.MethodGet)
	s.router.HandleFunc("/api/setup", s.setupPost()).Methods(http.MethodPost)
	s.router.HandleFunc("/api/setup/test-connection", s.setupTestConnectionPost()).Methods(http.MethodPost)

	authenticatedApis := s.router.PathPrefix("/api").Subrouter()
	authenticatedApis.Use(s.requireAuthentication)
	authenticatedApis.HandleFunc("/entry", s.entryPost()).Methods(http.MethodPost)
	authenticatedApis.HandleFunc("/entry/{id}", s.entryPut()).Methods(http.MethodPut)
	authenticatedApis.HandleFunc("/entry/{id}", s.entryDelete()).Methods(http.MethodDelete)
	authenticatedApis.HandleFunc("/guest-links", s.guestLinksPost()).Methods(http.MethodPost)
	authenticatedApis.HandleFunc("/guest-links/{id}", s.guestLinksDelete()).Methods(http.MethodDelete)
	authenticatedApis.HandleFunc("/guest-links/{id}/enable", s.guestLinksEnableDisable()).Methods(http.MethodPut)
	authenticatedApis.HandleFunc("/guest-links/{id}/disable", s.guestLinksEnableDisable()).Methods(http.MethodPut)

	adminApis := s.router.PathPrefix("/api").Subrouter()
	adminApis.Use(s.requireAuthentication)
	adminApis.Use(s.requireAdmin)
	adminApis.HandleFunc("/settings", s.settingsPut()).Methods(http.MethodPut)
	adminApis.HandleFunc("/maintenance/cleanup", s.cleanupPost()).Methods(http.MethodPost)
	adminApis.HandleFunc("/admin/oidc-settings", s.adminOIDCSettingsGet()).Methods(http.MethodGet)
	adminApis.HandleFunc("/admin/oidc-settings", s.adminOIDCSettingsPut()).Methods(http.MethodPut)
	adminApis.HandleFunc("/admin/oidc-settings/test-connection", s.adminOIDCSettingsTestConnectionPost()).Methods(http.MethodPost)
	adminApis.HandleFunc("/admin/users/{id}/grant-admin", s.memberGrantAdminPut()).Methods(http.MethodPut)
	adminApis.HandleFunc("/admin/users/{id}/revoke-admin", s.memberRevokeAdminPut()).Methods(http.MethodPut)

	publicApis := s.router.PathPrefix("/api").Subrouter()
	publicApis.HandleFunc("/guest/{guestLinkID}", s.guestEntryPost()).Methods(http.MethodPost)

	static := s.router.PathPrefix("/").Subrouter()
	static.PathPrefix("/css/").HandlerFunc(serveStaticResource()).Methods(http.MethodGet)
	static.PathPrefix("/js/").HandlerFunc(serveStaticResource()).Methods(http.MethodGet)
	static.PathPrefix("/third-party/").HandlerFunc(serveStaticResource()).Methods(http.MethodGet)

	// Add all the root-level static resources.
	for _, f := range []string{
		"/android-chrome-192x192.png",
		"/android-chrome-384x384.png",
		"/apple-touch-icon.png",
		"/browserconfig.xml",
		"/favicon-16x16.png",
		"/favicon-32x32.png",
		"/favicon.ico",
		"/mstile-150x150.png",
		"/safari-pinned-tab.svg",
		"/site.webmanifest",
	} {
		static.Path(f).HandlerFunc(serveStaticResource()).Methods(http.MethodGet)
	}

	authenticatedViews := s.router.PathPrefix("/").Subrouter()
	authenticatedViews.Use(s.requireAuthentication)
	authenticatedViews.Use(s.resolveLanguage)
	authenticatedViews.Use(enforceContentSecurityPolicy)
	authenticatedViews.HandleFunc("/files", s.fileIndexGet()).Methods(http.MethodGet)
	authenticatedViews.HandleFunc("/files/{id}/downloads", s.fileDownloadsGet()).Methods(http.MethodGet)
	authenticatedViews.HandleFunc("/files/{id}/edit", s.fileEditGet()).Methods(http.MethodGet)
	authenticatedViews.HandleFunc("/files/{id}/info", s.fileInfoGet()).Methods(http.MethodGet)
	authenticatedViews.HandleFunc("/files/{id}/confirm-delete", s.fileConfirmDeleteGet()).Methods(http.MethodGet)
	authenticatedViews.HandleFunc("/guest-links", s.guestLinkIndexGet()).Methods(http.MethodGet)
	authenticatedViews.HandleFunc("/guest-links/new", s.guestLinksNewGet()).Methods(http.MethodGet)

	adminViews := s.router.PathPrefix("/").Subrouter()
	adminViews.Use(s.requireAuthentication)
	adminViews.Use(s.requireAdmin)
	adminViews.Use(s.resolveLanguage)
	adminViews.Use(enforceContentSecurityPolicy)
	adminViews.HandleFunc("/information", s.systemInformationGet()).Methods(http.MethodGet)
	adminViews.HandleFunc("/settings", s.settingsGet()).Methods(http.MethodGet)
	adminViews.HandleFunc("/files/all", s.fileAllGet()).Methods(http.MethodGet)
	adminViews.HandleFunc("/members", s.membersGet()).Methods(http.MethodGet)
	adminViews.HandleFunc("/sso-settings", s.ssoSettingsGet()).Methods(http.MethodGet)

	views := s.router.PathPrefix("/").Subrouter()
	views.Use(upgradeToHttps)
	views.Use(s.resolveLanguage)
	views.Use(enforceContentSecurityPolicy)
	views.HandleFunc("/login", s.authGet()).Methods(http.MethodGet)
	views.HandleFunc("/oidc/callback", s.oidcCallbackGet()).Methods(http.MethodGet)
	views.HandleFunc("/setup", s.setupGet()).Methods(http.MethodGet)
	views.PathPrefix("/g/{guestLinkID}").HandlerFunc(s.guestUploadGet()).Methods(http.MethodGet)
	views.HandleFunc("/", s.indexGet()).Methods(http.MethodGet)
	// The unlock and preview routes must precede the /-{id} prefix routes
	// below, which would otherwise match them first.
	views.HandleFunc("/-{id}/unlock", s.entryUnlockGet()).Methods(http.MethodGet)
	views.HandleFunc("/-{id}/unlock", s.entryUnlockPost()).Methods(http.MethodPost)
	views.HandleFunc("/-{id}/preview", s.entryPreviewGet()).Methods(http.MethodGet)
	views.PathPrefix("/-{id}").HandlerFunc(s.entryGet()).Methods(http.MethodGet)
	views.PathPrefix("/-{id}/{filename}").HandlerFunc(s.entryGet()).Methods(http.MethodGet)

	s.addDevRoutes()
}
