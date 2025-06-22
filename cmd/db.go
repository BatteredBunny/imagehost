package cmd

import (
	"context"
	"errors"
	"strconv"
	"time"

	"crypto/rand"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
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

	GithubID       uint
	GithubUsername string

	AccountType string // Either "USER" or "ADMIN"
}

type UploadTokens struct {
	gorm.Model

	ID uint `gorm:"primaryKey"`

	Token uuid.UUID `gorm:"uniqueIndex"`

	AccountID uint
	Account   Accounts `gorm:"foreignKey:AccountID"`
}

type SessionTokens struct {
	gorm.Model

	ID uint `gorm:"primaryKey"`

	ExpiryDate time.Time
	Token      uuid.UUID `gorm:"uniqueIndex"`

	AccountID uint
	Account   Accounts `gorm:"foreignKey:AccountID"`
}

type Images struct {
	gorm.Model

	ID uint `gorm:"primaryKey"`

	FileName string
	FileSize uint
	MimeType string

	ExpiryDate time.Time `gorm:"default:null"` // Time when the image will be deleted

	UploaderID uint
	Uploader   Accounts `gorm:"foreignKey:UploaderID"`
}

type InviteCodes struct {
	gorm.Model

	ID          uint // Internal numeric image id
	Code        string
	Uses        uint // How many usages of this code is left
	ExpiryDate  time.Time
	AccountType string // Either registers normal or admin users

	InviteCreatorID uint     `gorm:"default:null"`
	InviteCreator   Accounts `gorm:"foreignKey:InviteCreatorID"`
}

var ErrInvalidDatabaseType = errors.New("Invalid database type")

func prepareDB(c Config) (database Database) {
	log.Info().Msg("Setting up database")

	var gormConnection gorm.Dialector
	if c.DatabaseType == "postgresql" {
		gormConnection = postgres.Open(c.DatabaseConnectionUrl)
	} else if c.DatabaseType == "sqlite" {
		gormConnection = sqlite.Open(c.DatabaseConnectionUrl)
	} else {
		log.Fatal().Err(ErrInvalidDatabaseType).Msg("Invalid database chosehn")
	}

	var err error
	database.DB, err = gorm.Open(gormConnection, &gorm.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database connection")
	}

	if err := database.DB.AutoMigrate(
		&Accounts{},
		&Images{},
		&InviteCodes{},
		&SessionTokens{},
		&UploadTokens{},
	); err != nil {
		log.Fatal().Err(err).Msg("Migration failed")
	}

	// Create the first admin user if no user with ID 1 exists
	userAmount, err := database.accountAmount()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get user amount")
	}
	inviteCodeAmount, err := database.inviteCodeAmount()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get invite amount")
	}

	if userAmount == 0 && inviteCodeAmount == 0 {
		inviteCode, err := database.createInviteCode(1, "ADMIN", 0)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create initial invite")
		}

		log.Warn().Msgf("No accounts found, please create your account via this registration token: %s", inviteCode.Code)
	}

	return
}

// Returns number of accounts in the database
func (db *Database) accountAmount() (count int64, err error) {
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
	return db.Model(&Accounts{}).
		Where(&Accounts{ID: accountID}).
		Update("github_username", username).Error
}

func (db *Database) linkGithub(userID uint, username string, rawGithubID string) (err error) {
	githubID, err := strconv.ParseUint(rawGithubID, 10, 0)
	if err != nil {
		return
	}

	return db.Model(&Accounts{}).
		Where(&Accounts{ID: userID}).
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
		Where("expiry_date > ?", time.Now()).
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
		ExpiryDate:      time.Now().Add(time.Hour * 24 * 7), // A week from now
	}

	err = db.Create(&inviteCode).Error

	return
}

func (db *Database) useCode(code string) (accountType string, err error) {
	var inviteCode InviteCodes
	if err = db.Model(&InviteCodes{}).
		Where(&InviteCodes{Code: code}).
		Where("expiry_date > ?", time.Now()).
		Where("uses > 0").
		First(&inviteCode).Error; err != nil {
		return
	}

	if err = db.Model(&InviteCodes{}).
		Where(&InviteCodes{ID: inviteCode.ID}).
		Update("uses", gorm.Expr("uses - 1")).Error; err != nil {
		return
	}

	accountType = inviteCode.AccountType

	return
}

func (db *Database) getUserBySessionToken(sessionToken uuid.UUID) (account Accounts, err error) {
	var accountID uint
	if err = db.Model(&SessionTokens{}).
		Where(&SessionTokens{Token: sessionToken}).
		Where("expiry_date > ?", time.Now()).
		Select("account_id").
		First(&accountID).Error; err != nil {
		return
	}

	err = db.Model(&Accounts{}).
		Where(&Accounts{ID: accountID}).
		First(&account).Error

	return
}

// Deletes image entry from database
func (db *Database) deleteImage(fileName string, uploadToken uuid.UUID) (err error) {
	account, err := db.getAccountByUploadToken(uploadToken)
	if err != nil {
		return
	}

	return db.Model(&Images{}).
		Where(&Images{FileName: fileName, UploaderID: account.ID}).
		Delete(&Images{}).Error
}

func (db *Database) getAccountByUploadToken(uploadToken uuid.UUID) (account Accounts, err error) {
	var accountID uint
	if err = db.Model(&UploadTokens{}).
		Where(&UploadTokens{Token: uploadToken}).
		Select("account_id").
		First(&accountID).Error; err != nil {
		return
	}

	err = db.Model(&Accounts{}).
		Where(&Accounts{ID: accountID}).
		First(&account).Error

	return
}

// Creates image entry in database, set the expiryDate to a future date when the image should be deleted
func (db *Database) createImageEntry(fileName string, fileSize uint, mimeType string, uploadToken uuid.UUID, expiryDate time.Time) (err error) {
	account, err := db.getAccountByUploadToken(uploadToken)
	if err != nil {
		return
	}

	// Insert new image
	return db.Model(&Images{}).Create(&Images{
		FileName:   fileName,
		FileSize:   fileSize,
		MimeType:   mimeType,
		UploaderID: account.ID,
		ExpiryDate: expiryDate,
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
	return db.Delete(&Accounts{}, userID).Error
}

func (db *Database) inviteCodesByAccount(accountID uint) (inviteCodes []InviteCodes, err error) {
	err = db.Model(&InviteCodes{}).
		Where("expiry_date > ?", time.Now()).
		Where("uses > 0").
		Where(&InviteCodes{InviteCreatorID: accountID}).
		Scan(&inviteCodes).Error

	return
}

func (db *Database) getAccounts() (users []Accounts, err error) {
	err = db.Model(&Accounts{}).
		Scan(&users).Error

	return
}

func (db *Database) imagesAmountOnAccount(accountID uint) (count int64, err error) {
	err = db.Model(&Images{}).
		Where(&Images{UploaderID: accountID}).
		Where("(expiry_date not null AND expiry_date > ?) OR expiry_date is null", time.Now()).
		Count(&count).Error

	return
}

func (db *Database) getAllImagesFromAccount(userID uint) (images []Images, err error) {
	err = db.Model(&Images{}).
		Where(&Images{UploaderID: userID}).
		Where("(expiry_date not null AND expiry_date > ?) OR expiry_date is null", time.Now()).
		Find(&images).Error

	return
}

// Looks if file exists in database
func (db *Database) fileExists(fileName string) (bool, error) {
	var count int64
	if err := db.Model(&Images{}).
		Where(&Images{FileName: fileName}).
		Where("(expiry_date not null AND expiry_date > ?) OR expiry_date is null", time.Now()).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (db *Database) createSessionToken(userID uint) (sessionToken uuid.UUID, err error) {
	log.Debug().Msgf("Creating session token for account %d", userID)

	session := SessionTokens{
		AccountID:  userID,
		Token:      uuid.New(),
		ExpiryDate: time.Now().Add(time.Hour * 24 * 7), // A week from now
	}

	if err = db.Model(&SessionTokens{}).Create(&session).Error; err != nil {
		return
	}

	sessionToken = session.Token

	return
}

var ErrInvalidAccountType = errors.New("Invalid account type specified")

func (db *Database) createAccount(accountType string) (account Accounts, err error) {
	if accountType == "ADMIN" || accountType == "USER" {
		account = Accounts{
			AccountType: accountType,
		}

		err = db.Model(&Accounts{}).Create(&account).Error
	} else {
		err = ErrInvalidAccountType
	}

	return
}

func (db *Database) getUploadTokens(userID uint) (uploadTokens []uuid.UUID, err error) {
	err = db.Model(&UploadTokens{}).
		Where(&UploadTokens{AccountID: userID}).
		Select("token").
		Scan(&uploadTokens).Error

	return
}

func (db *Database) createUploadToken(userID uint) (uploadToken uuid.UUID, err error) {
	uploadToken = uuid.New()

	err = db.Model(&UploadTokens{}).
		Create(&UploadTokens{AccountID: userID, Token: uploadToken}).Error

	return
}

func (db *Database) findExpiredImages() (images []Images, err error) {
	err = db.Model(&Images{}).
		Where("expiry_date not null AND expiry_date < ?", time.Now()).
		Find(&images).Error

	return
}

func (db *Database) deleteExpiredImages() (err error) {
	return db.Model(&Images{}).
		Where("expiry_date not null AND expiry_date < ?", time.Now()).
		Delete(&Images{}).Error
}

func (db *Database) deleteExpiredSessionTokens() (err error) {
	return db.Model(&SessionTokens{}).
		Where("expiry_date not null AND expiry_date < ?", time.Now()).
		Delete(&SessionTokens{}).Error
}

func (db *Database) deleteExpiredInviteCodes() (err error) {
	return db.Model(&InviteCodes{}).
		Where("expiry_date not null AND expiry_date < ?", time.Now()).
		Delete(&InviteCodes{}).Error
}
