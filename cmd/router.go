package cmd

import (
	"embed"
	"html/template"
	"time"

	"github.com/didip/tollbooth/v8"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/gin-gonic/gin"
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
	app.logInfo.Println("Setting up router")

	app.Router = gin.Default()
	app.Router.ForwardedByClientIP = c.behindReverseProxy
	app.Router.SetTrustedProxies([]string{c.trustedProxy})

	app.Router.SetHTMLTemplate(template.Must(template.ParseFS(TemplateFiles, "templates/*.gohtml")))

	app.Router.Use(
		app.bodySizeMiddleware(),
	)

	api := app.Router.Group("/api")
	api.Use(app.apiMiddleware())

	app.setupAuth(api)

	// Apis that require the upload token, typical this token is included in scripts
	fileAPI := api.Group("/file")
	fileAPI.Use(
		hasUploadTokenMiddleware(),
		app.uploadTokenVerificationMiddleware(),
	)

	fileAPI.POST("/upload", app.uploadImageAPI)
	fileAPI.POST("/delete", app.deleteImageAPI)
	// ---

	// Accounts for managing your user
	accountAPI := api.Group("/account")
	accountAPI.Use(
		hasTokenMiddleware(),
		app.userTokenVerificationMiddleware(),
	)

	accountAPI.POST("/new_upload_token", app.newUploadTokenAPI)
	accountAPI.POST("/delete", app.accountDeleteAPI)
	// ---

	// Admin apis
	adminAPI := api.Group("/admin")
	adminAPI.Use(
		hasTokenMiddleware(),
		app.adminTokenVerificationMiddleware(),
	)

	adminAPI.POST("/create_user", app.adminCreateUser)
	adminAPI.POST("/delete_user", app.adminDeleteUser)
	// ---

	app.Router.GET("/api_list", app.apiList)
	app.Router.StaticFS("/public/", PublicFiles())

	app.Router.GET("/login", app.loginPage)
	app.Router.GET("/logout", app.logoutHandler)
	app.Router.GET("/", app.indexPage)
	app.Router.GET("/user", app.userPage)
	app.Router.Use(app.ratelimitMiddleware())
	app.Router.NoRoute(app.indexFiles)

	return
}
