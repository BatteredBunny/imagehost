package cmd

import (
	"embed"
	"html/template"
	"time"

	"github.com/didip/tollbooth/v8"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func setupRatelimiting(c Config) *limiter.Limiter {
	rateLimiter := tollbooth.NewLimiter(2, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})

	if c.behindReverseProxy {
		rateLimiter.SetIPLookup(limiter.IPLookup{
			Name: "X-Forwarded-For",
		})
	} else {
		rateLimiter.SetIPLookup(limiter.IPLookup{
			Name: "RemoteAddr",
		})
	}

	return rateLimiter
}

//go:embed templates
var TemplateFiles embed.FS

func setupRouter(uninitializedApp *uninitializedApplication, c Config) (app *Application) {
	app = (*Application)(uninitializedApp)
	log.Info().Msg("Setting up router")

	app.Router = gin.Default()
	app.Router.ForwardedByClientIP = c.behindReverseProxy
	app.Router.SetTrustedProxies([]string{c.trustedProxy})

	templates := template.Must(template.New("").Funcs(template.FuncMap{
		"formatTimeDate": formatTimeDate,
		"mimeIsImage":    mimeIsImage,
		"mimeIsVideo":    mimeIsVideo,
		"mimeIsAudio":    mimeIsAudio,
	}).ParseFS(TemplateFiles, "templates/*.gohtml", "templates/components/*.gohtml"))
	app.Router.SetHTMLTemplate(templates)

	app.Router.Use(
		app.bodySizeMiddleware(),
	)

	api := app.Router.Group("/api")
	api.Use(app.apiMiddleware())

	app.setupAuth(api)

	// Apis that require the upload token, typical this token is included in scripts
	fileAPI := api.Group("/file")
	fileAPI.Use(
		app.hasUploadOrSessionTokenMiddleware(),
	)

	fileAPI.POST("/upload", app.uploadImageAPI)
	fileAPI.POST("/delete", app.deleteImageAPI)
	// ---

	// Accounts for managing your user
	accountAPI := api.Group("/account")
	accountAPI.Use(
		app.verifySessionAuthentication(),
		app.isSessionAuthenticated(),
	)

	accountAPI.POST("/delete", app.accountDeleteAPI)
	accountAPI.POST("/new_upload_token", app.newUploadTokenApi)
	accountAPI.POST("/new_invite_code", app.newInviteCodeApi)
	accountAPI.POST("/delete_images", app.deleteImagesAPI)
	// ---

	// Admin apis
	adminAPI := api.Group("/admin")
	adminAPI.Use(
		app.verifySessionAuthentication(),
		app.isAdmin(),
	)

	adminAPI.POST("/delete_user", app.adminDeleteUser)

	app.Router.StaticFS("/public/", PublicFiles())

	app.Router.GET("/login", app.loginPage)
	app.Router.GET("/register", app.registerPage)
	app.Router.GET("/logout", app.logoutHandler)
	app.Router.GET("/user", app.userPage)
	app.Router.GET("/admin", app.adminPage)
	app.Router.GET("/", app.indexPage)
	app.Router.Use(app.ratelimitMiddleware())
	app.Router.NoRoute(app.indexFiles)

	return
}
