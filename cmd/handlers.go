package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

func (app *Application) indexPage(c *gin.Context) {
	templateInput := gin.H{
		"Host": c.Request.Host,
	}

	_, account, loggedIn, err := app.validateAuthCookie(c)
	if errors.Is(err, ErrInvalidAuthCookie) {
		app.clearAuthCookie(c)
	} else if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if loggedIn {
		// For top bar
		templateInput["LoggedIn"] = true
		templateInput["AccountID"] = account.ID
		templateInput["IsAdmin"] = account.AccountType == "ADMIN"
	}

	c.HTML(http.StatusOK, "index.gohtml", templateInput)
}

type AccountStats struct {
	Accounts
	SpaceUsed     int64
	InvitedBy     string
	FilesUploaded int64
	You           bool
}

func (app *Application) toAccountStats(account *Accounts, requesterAccountID uint) (stats AccountStats, err error) {
	images, err := app.db.getAllImagesFromAccount(account.ID)
	if err != nil {
		return
	}

	stats = AccountStats{
		Accounts:      *account,
		SpaceUsed:     0,
		InvitedBy:     "",
		FilesUploaded: 0,
		You:           account.ID == requesterAccountID,
	}

	if account.InvitedBy == 0 {
		stats.InvitedBy = "system"
	} else if account.InvitedBy > 0 {
		invitedBy, err := app.db.getAccountByID(account.InvitedBy)
		if err == nil && invitedBy.GithubUsername != "" {
			stats.InvitedBy = fmt.Sprintf("%s (%d)", invitedBy.GithubUsername, invitedBy.ID)
		} else {
			stats.InvitedBy = strconv.Itoa(int(account.InvitedBy))
		}
	} else {
		stats.InvitedBy = strconv.Itoa(int(account.InvitedBy))
	}

	for _, image := range images {
		stats.SpaceUsed += int64(image.FileSize)
		stats.FilesUploaded++
	}

	if stats.GithubUsername == "" {
		stats.GithubUsername = "none"
	}

	return
}

func (app *Application) adminPage(c *gin.Context) {
	_, account, loggedIn, err := app.validateAuthCookie(c)
	if errors.Is(err, ErrInvalidAuthCookie) {
		app.clearAuthCookie(c)
	} else if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if account.AccountType != "ADMIN" {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
		return
	}

	var templateInput gin.H = make(gin.H)

	if loggedIn {
		// For top bar
		templateInput["LoggedIn"] = true
		templateInput["AccountID"] = account.ID
		templateInput["IsAdmin"] = account.AccountType == "ADMIN"

		users, err := app.db.getAccounts()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		var stats []AccountStats
		for _, user := range users {
			stat, err := app.toAccountStats(&user, account.ID)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			stats = append(stats, stat)
		}

		templateInput["Users"] = stats
	}

	if loggedIn {
		c.HTML(http.StatusOK, "admin.gohtml", templateInput)
	} else {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
	}
}

func (app *Application) userPage(c *gin.Context) {
	_, account, loggedIn, err := app.validateAuthCookie(c)
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

		templateInput["UnlinkedAccount"] = account.GithubID == 0

		templateInput["InviteCodes"], err = app.db.inviteCodesByAccount(account.ID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		var images []Images
		images, err = app.db.getAllImagesFromAccount(account.ID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		templateInput["Files"] = images
		templateInput["ImagesCount"] = len(images)
		templateInput["ImagesSize"] = Sum(images, func(image Images) int {
			return int(image.FileSize)
		})

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
	_, _, loggedIn, err := app.validateAuthCookie(c)
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
	_, _, loggedIn, err := app.validateAuthCookie(c)
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
	sessionToken, exists := c.Get("sessionToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getAccountBySessionToken(sessionToken.(uuid.UUID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to fetch user by session token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var uploadToken uuid.UUID

	nickname := c.PostForm("nickname")

	if uploadToken, err = app.db.createUploadToken(account.ID, nickname); err != nil {
		log.Err(err).Msg("Failed to create upload token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, uploadToken.String())
}

func (app *Application) deleteUploadTokenAPI(c *gin.Context) {
	sessionToken, exists := c.Get("sessionToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getAccountBySessionToken(sessionToken.(uuid.UUID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to fetch user by session token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	rawUploadToken := c.PostForm("upload_token")
	if rawUploadToken == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	uploadToken, err := uuid.Parse(rawUploadToken)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if err = app.db.deleteUploadToken(account.ID, uploadToken); err != nil {
		log.Err(err).Msg("Failed to delete upload token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Upload token deleted successfully")
}

// TODO: allow specifying uses and if its an admin account allow creating admin invites
func (app *Application) newInviteCodeApi(c *gin.Context) {
	sessionToken, exists := c.Get("sessionToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getAccountBySessionToken(sessionToken.(uuid.UUID))
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

func (app *Application) deleteInviteCodeAPI(c *gin.Context) {
	sessionToken, exists := c.Get("sessionToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getAccountBySessionToken(sessionToken.(uuid.UUID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to fetch user by session token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	inviteCode := c.PostForm("invite_code")

	if err = app.db.deleteInviteCode(inviteCode, account.ID); err != nil {
		log.Err(err).Msg("Failed to delete invite code")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Invite code deleted successfully")
}

func (app *Application) deleteImagesAPI(c *gin.Context) {
	sessionToken, exists := c.Get("sessionToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getAccountBySessionToken(sessionToken.(uuid.UUID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to fetch user by session token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	images, err := app.db.getAllImagesFromAccount(account.ID)
	if err != nil {
		return
	}

	for _, image := range images {
		if err = app.deleteFile(image.FileName); err != nil {
			log.Err(err).Msg("Failed to delete image")
		}
	}

	if err := app.db.deleteImagesFromAccount(account.ID); err != nil {
		log.Err(err).Msg("Failed to delete images")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Images deleted")
}
