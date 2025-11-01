package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/markbates/goth"
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

	SpaceUsed         uint
	InvitedBy         string
	FilesUploaded     int64
	You               bool
	SessionsCount     int64
	UploadTokensCount int64
	LastActivity      time.Time // Last session or upload token usage
}

func (app *Application) toAccountStats(account *Accounts, requesterAccountID uint) (stats AccountStats, err error) {
	files, err := app.db.getAllFilesFromAccount(account.ID)
	if err != nil {
		return
	}

	stats = AccountStats{
		Accounts: *account,
		You:      account.ID == requesterAccountID,
	}

	stats.SessionsCount, err = app.db.getSessionsCount(account.ID)
	if err != nil {
		log.Err(err).Msg("Failed to get session count")
	}

	stats.UploadTokensCount, err = app.db.getUploadTokensCount(account.ID)
	if err != nil {
		log.Err(err).Msg("Failed to get upload token count")
	}

	stats.LastActivity, err = app.db.lastAccountActivity(account.ID)
	if err != nil {
		log.Err(err).Msg("Failed to get last activity")
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

	for _, file := range files {
		stats.SpaceUsed += file.FileSize
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

		templateInput["MaxUploadSize"] = uint(app.config.MaxUploadSize)
	}

	if loggedIn {
		c.HTML(http.StatusOK, "admin.gohtml", templateInput)
	} else {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
	}
}

type UiFile struct {
	Files
	Views uint
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

		var rawFiles []Files
		rawFiles, err = app.db.getAllFilesFromAccount(account.ID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		var files []UiFile
		for _, file := range rawFiles {
			count, err := app.db.getFileViews(file.ID)
			if err != nil {
				log.Err(err).Msg("Failed to determine file views")
			}

			files = append(files, UiFile{Files: file, Views: uint(count)})
		}

		templateInput["Files"] = files
		templateInput["FilesCount"] = len(files)
		templateInput["FilesSizeTotal"] = uint(Sum(files, func(file UiFile) int {
			return int(file.FileSize)
		}))

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

	var providers []string
	for _, provider := range goth.GetProviders() {
		providers = append(providers, provider.Name())
	}

	if loggedIn {
		c.Redirect(http.StatusTemporaryRedirect, "/user")
	} else {
		c.HTML(http.StatusOK, "login.gohtml", gin.H{
			"Providers": providers,
		})
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

	// Probably better ways to do this
	if strings.HasPrefix(c.Request.URL.Path, "/api") {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Looks if file exists in public folder then redirects there
	filePath := filepath.Join("public", path.Clean(c.Request.URL.Path))
	if file, err := publicFiles.Open(filePath); err == nil {
		file.Close()
		c.Redirect(http.StatusPermanentRedirect, path.Join("public", path.Clean(c.Request.URL.Path)))
		return
	}

	// Looks in database for uploaded file
	fileName := path.Base(path.Clean(c.Request.URL.Path))
	if exists, err := app.db.fileExists(fileName); err != nil {
		log.Err(err).Msg("Failed to check if file exists")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	} else if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		return
	}

	if err := app.db.bumpFileViews(fileName, c.ClientIP()); err != nil {
		log.Err(err).Msg("Failed to bump file views")
	}

	switch app.config.FileStorageMethod {
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

func (app *Application) deleteFilesAPI(c *gin.Context) {
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

	if err = app.deleteFilesFromAccount(account.ID); err != nil {
		log.Err(err).Msg("Failed to delete files from account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Files deleted")
}

func (app *Application) deleteFilesFromAccount(userID uint) (err error) {
	files, err := app.db.getAllFilesFromAccount(userID)
	if err != nil {
		return
	}

	if err = app.db.deleteFilesFromAccount(userID); err != nil {
		return
	}

	for _, file := range files {
		if err = app.deleteFile(file.FileName); err != nil {
			log.Err(err).Msg("Failed to delete file")
		}
	}

	return
}

type FilesApiInput struct {
	Skip uint   `form:"skip,default=0"`          // Used for pagination
	Sort string `form:"sort,default=created_at"` // "created_at", "views", "file_size"
}

type FilesApiOutput struct {
	Files []Files `json:"files"`
	Count int64   `json:"count"`
}

func (app *Application) filesAPI(c *gin.Context) {
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

	var input FilesApiInput
	if err = c.MustBindWith(&input, binding.Form); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var allowedSorts = []string{
		"created_at",
		"views",
		"file_size",
	}
	if !slices.Contains(allowedSorts, input.Sort) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Api returns 5 files at a time
	var limit uint = 5
	desc := true

	var output FilesApiOutput
	output.Files, err = app.db.getFilesPaginatedFromAccount(account.ID, input.Skip, limit, input.Sort, desc)
	if err != nil {
		log.Err(err).Msg("Failed to get files from account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	count, err := app.db.filesAmountOnAccount(account.ID)
	if err != nil {
		log.Err(err).Msg("Failed to get files amount on account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	output.Count = count

	c.JSON(http.StatusOK, output)
}
