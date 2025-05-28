package cmd

import (
	"context"
	"errors"
	"strconv"
	"time"

	"crypto/rand"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database struct {
	*gorm.DB
}

type Accounts struct {
	gorm.Model

	ID uint `gorm:"primaryKey"` // Internal numeric account ID

	// TODO: allow multiple tokens
	UploadToken uuid.UUID `gorm:"uniqueIndex"` // Token that authorizes uploads for that account

	GithubID       uint
	GithubUsername string

	AccountType string // Either "USER" or "ADMIN"
}

type SessionTokens struct {
	gorm.Model

	ID uint `gorm:"primaryKey"`

	ExpiryDate time.Time // TODO: implement
	Token      uuid.UUID `gorm:"uniqueIndex"`

	AccountID uint
	Account   Accounts `gorm:"foreignKey:AccountID"`
}

type Images struct {
	gorm.Model

	ID uint `gorm:"primaryKey"`

	FileName   string
	ExpiryDate time.Time // Time when the image will be deleted

	UploaderID uint
	Uploader   Accounts `gorm:"foreignKey:UploaderID"`
}

type InviteCodes struct {
	gorm.Model

	ID          uint // Internal numeric image id
	Code        string
	Uses        uint      // How many usages of this code is left
	ExpiryDate  time.Time // TODO: implement
	AccountType string    // Either registers normal or admin users

	InviteCreatorID uint
	InviteCreator   Accounts `gorm:"foreignKey:InviteCreatorID"`
}

var ErrInvalidDatabaseType = errors.New("Invalid database type")

func prepareDB(l *Logger, c Config) (database Database) {
	l.logInfo.Println("Setting up database")

	var gormConnection gorm.Dialector
	if c.DatabaseType == "postgresql" {
		gormConnection = postgres.Open(c.DatabaseConnectionUrl)
	} else if c.DatabaseType == "sqlite" {
		gormConnection = sqlite.Open(c.DatabaseConnectionUrl)
	} else {
		l.logError.Fatal(ErrInvalidDatabaseType)
	}

	var err error
	database.DB, err = gorm.Open(gormConnection, &gorm.Config{})
	if err != nil {
		l.logError.Fatal(err)
	}

	if err := database.DB.AutoMigrate(
		&Accounts{},
		&Images{},
		&InviteCodes{},
		&SessionTokens{},
	); err != nil {
		l.logError.Fatal(err)
	}

	// Create the first admin user if no user with ID 1 exists
	userAmount, err := database.userAmount()
	if err != nil {
		l.logError.Fatal(err)
	}
	inviteCodeAmount, err := database.inviteCodeAmount()
	if err != nil {
		l.logError.Fatal(err)
	}

	if userAmount == 0 && inviteCodeAmount == 0 {
		inviteCode, err := database.createInviteCode(1, "ADMIN", 0)
		if err != nil {
			l.logError.Fatal(err)
		}

		l.logWarning.Println("No accounts found, please create your account via this registration token:", inviteCode.Code)
	}

	return
}

func (db *Database) userAmount() (count int64, err error) {
	err = db.Model(&Accounts{}).
		Count(&count).Error

	return
}

func (db *Database) findAccountByGithubID(rawID string) (account Accounts, err error) {
	id, err := strconv.ParseUint(rawID, 10, 0)
	if err != nil {
		return
	}

	if err = db.Model(&Accounts{}).
		Where(&Accounts{GithubID: uint(id)}).
		First(&account).Error; err != nil {
		return
	}

	return
}

func (db *Database) updateGithubUsername(accountID uint, username string) (err error) {
	return db.Model(&Accounts{ID: accountID}).
		Update("github_username", username).Error
}

func (db *Database) linkGithub(userID uint, username string, rawGithubID string) (err error) {
	githubID, err := strconv.ParseUint(rawGithubID, 10, 0)
	if err != nil {
		return
	}

	return db.Model(&Accounts{ID: userID}).
		Updates(map[string]interface{}{
			"github_username": username,
			"github_id":       uint(githubID),
		}).Error
}

func (db *Database) deleteSession(sessionToken uuid.UUID) (err error) {
	return db.Model(&SessionTokens{}).
		Where(&SessionTokens{Token: sessionToken}).
		Delete(&SessionTokens{}).Error
}

func (db *Database) inviteCodeAmount() (count int64, err error) {
	err = db.Model(&InviteCodes{}).
		Where("uses > 0").
		Count(&count).Error

	return
}

func (db *Database) createInviteCode(uses uint, accountType string, inviteCreatorID uint) (inviteCode InviteCodes, err error) {
	inviteCode = InviteCodes{
		Code:            rand.Text(),
		Uses:            uses,
		AccountType:     accountType,
		InviteCreatorID: inviteCreatorID,
	}

	err = db.Create(&inviteCode).Error

	return
}

func (db *Database) useCode(code string) (accountType string, err error) {
	var inviteCode InviteCodes
	if err = db.Model(&InviteCodes{}).
		Where(&InviteCodes{Code: code}).
		Where("uses > 0").
		First(&inviteCode).Error; err != nil {
		return
	}

	if err = db.Model(&inviteCode).
		Update("uses", gorm.Expr("uses - 1")).Error; err != nil {
		return
	}

	accountType = inviteCode.AccountType

	return
}

func (db *Database) getUserBySessionToken(sessionToken uuid.UUID) (account Accounts, err error) {
	// TODO: make sure token isnt expired

	var accountID uint
	if err = db.Model(&SessionTokens{}).
		Where(&SessionTokens{Token: sessionToken}).
		Select("account_id").
		First(&accountID).Error; err != nil {
		return
	}

	err = db.Model(&Accounts{ID: accountID}).
		First(&account).Error

	return
}

// Deletes image entry from database
func (db *Database) deleteImage(fileName string, uploadToken uuid.UUID) (err error) {
	return db.Model(&Images{}).
		Where(&Images{FileName: fileName, Uploader: Accounts{UploadToken: uploadToken}}).
		Delete(&Images{}).Error
}

func (db *Database) insertNewImageUploadToken(fileName string, uploadToken uuid.UUID) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// First get the user ID by upload token
	var userID uint
	err = db.WithContext(ctx).Model(&Accounts{}).
		Select("id").
		Where(&Accounts{UploadToken: uploadToken}).
		First(&userID).Error
	if err != nil {
		return
	}

	// Insert new image
	return db.WithContext(ctx).Create(&Images{
		FileName:   fileName,
		UploaderID: userID,
	}).Error
}

func (db *Database) deleteImagesFromAccount(userID uint) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return db.WithContext(ctx).
		Where(&Images{UploaderID: userID}).
		Delete(&Images{}).Error
}

func (db *Database) deleteAccount(userID uint) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return db.WithContext(ctx).Delete(&Accounts{}, userID).Error
}

func (db *Database) imagesOnAccount(accountID uint) (count int64, err error) {
	err = db.Model(&Images{}).
		Where(&Images{UploaderID: accountID}).
		Count(&count).Error

	return
}

func (db *Database) getAllImagesFromAccount(userID uint) (images []Images, err error) {
	err = db.Model(&Images{}).
		Select("file_name").
		Where(&Images{UploaderID: userID}).
		Find(&images).Error

	return
}

// Looks if file exists in database
func (db *Database) fileExists(fileName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var count int64
	err := db.WithContext(ctx).Model(&Images{}).
		Where(&Images{FileName: fileName}).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (db *Database) createSessionToken(userID uint) (sessionToken uuid.UUID, err error) {
	session := SessionTokens{
		AccountID: userID,
		Token:     uuid.New(),
	}
	if err = db.Model(&SessionTokens{}).Create(&session).Error; err != nil {
		return
	}

	sessionToken = session.Token

	return
}

// gets user id by upload token
func (db *Database) idByUploadToken(uploadToken uuid.UUID) (id int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).Model(&Accounts{}).
		Select("id").
		Where(&Accounts{UploadToken: uploadToken}).
		First(&id).Error

	return
}

func (db *Database) replaceUploadToken(sessionToken uuid.UUID) (uploadToken string, err error) {
	account, err := db.getUserBySessionToken(sessionToken)
	if err != nil {
		return
	}

	if err = db.Model(&account).
		Update("upload_token", uuid.New()).Error; err != nil {
		return
	}

	uploadToken = account.UploadToken.String()

	return
}

var ErrInvalidAccountType = errors.New("Invalid account type specified")

func (db *Database) createAccount(accountType string) (account Accounts, err error) {
	if accountType == "ADMIN" || accountType == "USER" {
		account = Accounts{
			AccountType: accountType,
			UploadToken: uuid.New(),
		}

		err = db.Model(&Accounts{}).Create(&account).Error
	} else {
		err = ErrInvalidAccountType
	}

	return
}

func (db *Database) findAllExpiredImages() (images []Images, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// TODO: use expiry date field
	err = db.WithContext(ctx).
		Where("created_date < ?", time.Now().AddDate(0, 0, -7)).
		Find(&images).Error

	return
}

func (db *Database) deleteAllExpiredImages() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// TODO: use expiry date field
	err = db.WithContext(ctx).
		Where("created_date < ?", time.Now().AddDate(0, 0, -7)).
		Delete(&Images{}).Error

	return
}
