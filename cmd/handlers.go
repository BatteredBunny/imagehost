package cmd

import (
	"errors"
	"net/http"
	"path"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

func (app *Application) indexPage(c *gin.Context) {
	templateInput := gin.H{
		"Host": c.Request.Host,
	}

	_, account, loggedIn, err := app.validateCookie(c)
	if errors.Is(err, ErrInvalidAuthCookie) {
		app.clearAuthCookie(c)
	} else if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if loggedIn {
		templateInput["LoggedIn"] = true
		templateInput["AccountID"] = account.ID
	}

	c.HTML(http.StatusOK, "index.gohtml", templateInput)
}

func (app *Application) userPage(c *gin.Context) {
	_, account, loggedIn, err := app.validateCookie(c)
	if errors.Is(err, ErrInvalidAuthCookie) {
		app.clearAuthCookie(c)
	} else if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var templateInput gin.H = make(gin.H)

	if loggedIn && account.GithubID > 0 {
		templateInput["LinkedWithGithub"] = true
		templateInput["GithubUsername"] = account.GithubUsername
	}

	if loggedIn {
		// For top bar
		templateInput["LoggedIn"] = true
		templateInput["AccountID"] = account.ID

		templateInput["IsAdmin"] = account.AccountType == "ADMIN"

		templateInput["InviteCodes"], err = app.db.inviteCodesByAccount(account.ID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		templateInput["ImagesCount"], err = app.db.imagesAmountOnAccount(account.ID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		templateInput["Files"], err = app.db.getAllImagesFromAccount(account.ID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		uploadTokens, err := app.db.getUploadTokens(account.ID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		templateInput["UploadTokens"] = uploadTokens
	}

	if loggedIn {
		c.HTML(http.StatusOK, "user.gohtml", templateInput)
	} else {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
	}
}

func (app *Application) loginPage(c *gin.Context) {
	_, _, loggedIn, err := app.validateCookie(c)
	if errors.Is(err, ErrInvalidAuthCookie) {
		app.clearAuthCookie(c)
	} else if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if loggedIn {
		c.Redirect(http.StatusTemporaryRedirect, "/user")
	} else {
		c.HTML(http.StatusOK, "login.gohtml", nil)
	}
}

func (app *Application) registerPage(c *gin.Context) {
	_, _, loggedIn, err := app.validateCookie(c)
	if errors.Is(err, ErrInvalidAuthCookie) {
		app.clearAuthCookie(c)
	} else if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if loggedIn {
		c.Redirect(http.StatusTemporaryRedirect, "/user")
	} else {
		c.HTML(http.StatusOK, "register.gohtml", nil)
	}
}

func (app *Application) indexFiles(c *gin.Context) {
	c.Status(http.StatusOK)

	// Looks if file exists in public folder then redirects there
	filePath := filepath.Join("public", path.Clean(c.Request.URL.Path))
	if file, err := publicFiles.Open(filePath); err == nil {
		file.Close()
		c.Redirect(http.StatusPermanentRedirect, path.Join("public", path.Clean(c.Request.URL.Path)))
		return
	}

	// Looks in database for uploaded file
	if exists, err := app.db.fileExists(path.Base(path.Clean(c.Request.URL.Path))); err != nil {
		log.Err(err).Msg("Failed to check if file exists")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	} else if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		return
	}

	switch app.config.fileStorageMethod {
	case fileStorageS3:
		c.Redirect(http.StatusTemporaryRedirect, "https://"+app.config.S3.CdnDomain+"/file/"+app.config.S3.Bucket+path.Clean(c.Request.URL.Path))
	case fileStorageLocal:
		c.File(filepath.Join(app.config.DataFolder, path.Clean(c.Request.URL.Path)))
	default:
		log.Err(ErrUnknownStorageMethod).Msg("No storage method chosen")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (app *Application) newUploadTokenApi(c *gin.Context) {
	sessionToken, exists := c.Get("token")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getUserBySessionToken(sessionToken.(uuid.UUID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to fetch user by session token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var uploadToken uuid.UUID
	if uploadToken, err = app.db.createUploadToken(account.ID); err != nil {
		log.Err(err).Msg("Failed to create upload token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, uploadToken.String())
}

// TOOD: allow specifying uses and if its an admin account allow creating admin invites
func (app *Application) newInviteCodeApi(c *gin.Context) {
	sessionToken, exists := c.Get("token")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getUserBySessionToken(sessionToken.(uuid.UUID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to fetch user by session token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	inviteCode, err := app.db.createInviteCode(5, "USER", account.ID)
	if err != nil {
		log.Err(err).Msg("Failed to create invite code")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, inviteCode.Code)
}
