package cmd

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Api for deleting your own account
func (app *Application) accountDeleteAPI(c *gin.Context) {
	token, exists := c.Get("token")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	account, err := app.db.getUserBySessionToken(token.(uuid.UUID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to fetch user by session token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err = app.deleteAccountWithImages(account.ID); err != nil {
		log.Err(err).Msg("Failed to delete own account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func (app *Application) deleteAccountWithImages(userID uint) (err error) {
	images, err := app.db.getAllImagesFromAccount(userID)
	if err != nil {
		return
	}

	for _, image := range images {
		if err = app.deleteFile(image.FileName); err != nil {
			log.Err(err).Msg("Failed to delete image")
		}
	}

	if err = app.db.deleteImagesFromAccount(userID); err != nil {
		return
	}

	if err = app.db.deleteAccount(userID); err != nil {
		return
	}

	return
}

// Api for deleting an image from your account
type deleteImageAPIInput struct {
	FileName string `form:"file_name"`
}

func (app *Application) deleteImageAPI(c *gin.Context) {
	var input deleteImageAPIInput
	var err error

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		c.Abort()
		return
	}

	// Makes sure the image exists
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

	// Should have been verified with middleware already
	uploadToken, exists := c.Get("uploadToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err = app.db.deleteImage(input.FileName, uploadToken.(uuid.UUID)); err != nil { // Deletes file entry from database
		log.Err(err).Msg("Failed to delete image entry")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Successfully deleted the image")
}

/*
Api for uploading image
curl -F 'upload_token=1234567890' -F 'file=@yourfile.png'
The expiry_date is a unix timestamp in seconds
*/
func (app *Application) uploadImageAPI(c *gin.Context) {
	var expiryDate time.Time
	timestamp, exists := c.GetPostForm("expiry_date")
	if exists {
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

	fullFileName := app.generateFullFileName(file)

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

	// Should have been verified with middleware already
	uploadToken, exists := c.Get("uploadToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err = app.db.createImageEntry(fullFileName, uploadToken.(uuid.UUID), expiryDate); errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Err(err).Msg("Failed to create image entry")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/"+fullFileName)
}
