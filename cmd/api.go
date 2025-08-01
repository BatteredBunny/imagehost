package cmd

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Api for deleting your own account
func (app *Application) accountDeleteAPI(c *gin.Context) {
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

	if err = app.deleteAccount(account.ID); err != nil {
		log.Err(err).Msg("Failed to delete own account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Account deleted successfully")
}

func (app *Application) deleteAccount(userID uint) (err error) {
	if err = app.db.deleteSessionTokensFromAccount(userID); err != nil {
		return
	}

	if err = app.db.deleteUploadTokensFromAccount(userID); err != nil {
		return
	}

	if err = app.db.deleteInviteCodesFromAccount(userID); err != nil {
		return
	}

	files, err := app.db.getAllFilesFromAccount(userID)
	if err != nil {
		return
	}

	for _, file := range files {
		if err = app.deleteFile(file.FileName); err != nil {
			log.Err(err).Msg("Failed to delete file")
		}
	}

	if err = app.db.deleteFilesFromAccount(userID); err != nil {
		return
	}

	if err = app.db.deleteAccount(userID); err != nil {
		return
	}

	return
}

// Api for deleting a file from your account
type deleteFileAPIInput struct {
	FileName string `form:"file_name"`
}

func (app *Application) deleteFileAPI(c *gin.Context) {
	var input deleteFileAPIInput
	var err error

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if input.FileName == "" {
		c.String(http.StatusBadRequest, "File name is required")
		c.Abort()
		return
	}

	rawSessionToken, sessionTokenExists := c.Get("sessionToken")
	rawUploadToken, uploadTokenExists := c.Get("uploadToken")

	var sessionToken uuid.NullUUID
	var uploadToken uuid.NullUUID
	if sessionTokenExists {
		sessionToken = uuid.NullUUID{UUID: rawSessionToken.(uuid.UUID), Valid: true}
	} else if uploadTokenExists {
		uploadToken = uuid.NullUUID{UUID: rawUploadToken.(uuid.UUID), Valid: true}
	} else {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Makes sure the file exists
	var exists bool
	if exists, err = app.db.fileExists(input.FileName); err != nil {
		log.Err(err).Msg("Failed to check if file exists")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	} else if !exists {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Deletes file
	if err = app.deleteFile(input.FileName); err != nil {
		log.Err(err).Msg("Failed to delete file")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err = app.db.deleteFile(input.FileName, uploadToken, sessionToken); err != nil { // Deletes file entry from database
		log.Err(err).Msg("Failed to delete file entry")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Successfully deleted the file")
}

/*
Api for uploading file
curl -F 'upload_token=1234567890' -F 'file=@yourfile.png'

Additional inputs:
expiry_timestamp: unix timestamp in seconds
expiry_date: YYYY-MM-DD in string, expiry_timestamp gets priority
*/
func (app *Application) uploadFileAPI(c *gin.Context) {
	var expiryDate time.Time

	date, exists := c.GetPostForm("expiry_date")
	if exists {
		expiryDate, _ = time.Parse("2006-01-02", date)
	}

	timestamp, exists := c.GetPostForm("expiry_timestamp")
	if exists {
		log.Info().Any("expiry_date", timestamp).Msg("Expiry date provided")
		unixSecs, err := strconv.Atoi(timestamp)
		if err == nil {
			expiryDate = time.Unix(int64(unixSecs), 0)
		}
	}

	fileRaw, _, err := c.Request.FormFile("file")
	defer fileRaw.Close()
	if err != nil {
		c.String(http.StatusBadRequest, "No file provided")
		c.Abort()
		return
	}

	file, err := io.ReadAll(fileRaw)
	if err != nil {
		log.Err(err).Msg("Failed to read uploaded file")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	mime := mimetype.Detect(file)
	fullFileName := app.generateFullFileName(mime)

	switch app.config.fileStorageMethod {
	case fileStorageS3:
		err = app.uploadFileS3(file, fullFileName)
	case fileStorageLocal:
		err = os.WriteFile(app.config.DataFolder+fullFileName, file, 0600)
	default:
		err = ErrUnknownStorageMethod
	}

	if err != nil {
		log.Err(err).Msg("Upload issue")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	input := CreateFileEntryInput{
		files: Files{
			FileName:   fullFileName,
			FileSize:   uint(len(file)),
			MimeType:   mime.String(),
			ExpiryDate: expiryDate,
		},
	}

	sessionToken, sessionTokenExists := c.Get("sessionToken")
	uploadToken, uploadTokenExists := c.Get("uploadToken")

	if sessionTokenExists {
		input.sessionToken = uuid.NullUUID{
			UUID:  sessionToken.(uuid.UUID),
			Valid: true,
		}
	} else if uploadTokenExists {
		input.uploadToken = uuid.NullUUID{
			UUID:  uploadToken.(uuid.UUID),
			Valid: true,
		}
	} else {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err = app.db.createFileEntry(input); errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to create file entry")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/"+fullFileName)
}
