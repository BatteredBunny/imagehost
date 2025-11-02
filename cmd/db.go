package cmd

import (
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Database struct {
	*gorm.DB
}

type Accounts struct {
	gorm.Model

	ID uint `gorm:"primaryKey"` // Internal numeric account ID

	GithubID       uint
	GithubUsername string

	InvitedBy uint // Account ID of the user who invited this account

	AccountType string // Either "USER" or "ADMIN"
}

type UploadTokens struct {
	gorm.Model

	ID uint `gorm:"primaryKey"`

	LastUsed *time.Time
	Nickname string

	Token uuid.UUID `gorm:"uniqueIndex"`

	AccountID uint
	Account   Accounts `gorm:"foreignKey:AccountID"`
}

type SessionTokens struct {
	gorm.Model

	ID uint `gorm:"primaryKey"`

	LastUsed   time.Time
	ExpiryDate time.Time
	Token      uuid.UUID `gorm:"uniqueIndex"`

	AccountID uint
	Account   Accounts `gorm:"foreignKey:AccountID"`
}

type Files struct {
	ID        uint `gorm:"primaryKey" json:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	FileName string // Newly generated file name

	OriginalFileName string // Original file name from upload
	FileSize         uint
	MimeType         string

	Public bool // If false, only the uploader can see the file

	Views      []FileViews `gorm:"foreignKey:FilesID" json:"-"`
	ViewsCount uint        `gorm:"-"` // Used for export

	ExpiryDate time.Time `gorm:"default:null"` // Time when the file will be deleted

	UploaderID uint     `json:"-"`
	Uploader   Accounts `gorm:"foreignKey:UploaderID" json:"-"`
}

type FileViews struct {
	gorm.Model

	// Each IP counts once as a view
	IpHash string `gorm:"index:,unique,composite:hash_collision"`

	FilesID uint `gorm:"index:,unique,composite:hash_collision"`
}

// Doesn't work
func (f *Files) AfterDelete(db *gorm.DB) (err error) {
	err = db.Model(&FileViews{}).
		Where("files_id = ?", f.ID).
		Delete(&FileViews{}).Error

	return
}

type InviteCodes struct {
	gorm.Model

	ID          uint `gorm:"primaryKey"`
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
		&Files{},
		&FileViews{},
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

func (db *Database) getFileViews(fileID uint) (count int64, err error) {
	err = db.Model(&FileViews{}).
		Where(&FileViews{FilesID: fileID}).
		Count(&count).Error

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

func (db *Database) deleteInviteCode(code string, accountID uint) (err error) {
	return db.Model(&InviteCodes{}).
		Where(&InviteCodes{
			Code:            code,
			InviteCreatorID: accountID,
		}).
		Delete(&InviteCodes{}).Error
}

func (db *Database) useCode(code string) (accountType string, invitedBy uint, err error) {
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
	invitedBy = inviteCode.InviteCreatorID

	return
}

// Returns the latest time a session token or an upload token was used
func (db *Database) lastAccountActivity(accountID uint) (lastActivity time.Time, err error) {
	var (
		sessionLastUsed sql.NullTime
		uploadLastUsed  sql.NullTime
	)

	if err = db.Model(&SessionTokens{}).
		Where(&SessionTokens{AccountID: accountID}).
		Select("last_used").
		Order("last_used DESC").
		First(&sessionLastUsed).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}

	// TODO: somehow hide the error in logs if no upload tokens exist
	if err = db.Model(&UploadTokens{}).
		Where(&UploadTokens{AccountID: accountID}).
		Select("last_used").
		Order("last_used DESC").
		First(&uploadLastUsed).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	} else if err != nil {
		return
	}

	if !sessionLastUsed.Valid && !uploadLastUsed.Valid {
		return time.Time{}, nil // No activity found
	} else if uploadLastUsed.Valid && uploadLastUsed.Time.After(sessionLastUsed.Time) {
		lastActivity = uploadLastUsed.Time
	} else {
		lastActivity = sessionLastUsed.Time
	}

	return
}

func (db *Database) getAccountBySessionToken(sessionToken uuid.UUID) (account Accounts, err error) {
	if err = db.Model(&SessionTokens{}).
		Where(&SessionTokens{Token: sessionToken}).
		Where("expiry_date > ?", time.Now()).
		Update("last_used", time.Now()).Error; err != nil {
		log.Err(err).Msg("Failed to update last used time for session token")
	}

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

// Deletes file entry from database
func (db *Database) deleteFileEntry(fileName string, uploadToken uuid.NullUUID, sessionToken uuid.NullUUID) (err error) {
	var account Accounts
	if uploadToken.Valid {
		account, err = db.getAccountByUploadToken(uploadToken.UUID)
		if err != nil {
			return
		}
	} else if sessionToken.Valid {
		account, err = db.getAccountBySessionToken(sessionToken.UUID)
		if err != nil {
			return
		}
	} else {
		// This shouldnt happen but just in case
		return ErrNotAuthenticated
	}

	return db.Model(&Files{}).
		Where(&Files{FileName: fileName, UploaderID: account.ID}).
		Delete(&Files{}).Error
}

func (db *Database) getAccountByUploadToken(uploadToken uuid.UUID) (account Accounts, err error) {
	var accountID uint
	if err = db.Model(&UploadTokens{}).
		Where(&UploadTokens{Token: uploadToken}).
		Select("account_id").
		First(&accountID).Error; err != nil {
		return
	}

	if err = db.Model(&UploadTokens{}).
		Where(&UploadTokens{Token: uploadToken}).
		Update("last_used", time.Now()).Error; err != nil {
		return
	}

	err = db.Model(&Accounts{}).
		Where(&Accounts{ID: accountID}).
		First(&account).Error

	return
}

type CreateFileEntryInput struct {
	files Files

	uploadToken  uuid.NullUUID
	sessionToken uuid.NullUUID
}

var ErrNotAuthenticated = errors.New("not authenticated")

// Creates file entry in database
func (db *Database) createFileEntry(input CreateFileEntryInput) (err error) {
	var account Accounts
	if input.sessionToken.Valid {
		account, err = db.getAccountBySessionToken(input.sessionToken.UUID)
		if err != nil {
			return
		}
	} else if input.uploadToken.Valid {
		account, err = db.getAccountByUploadToken(input.uploadToken.UUID)
		if err != nil {
			return
		}
	} else {
		// This shouldnt happen but just in case
		return ErrNotAuthenticated
	}

	input.files.UploaderID = account.ID

	return db.Model(&Files{}).Create(&input.files).Error
}

// Only deletes database entry, actual file has to be deleted as well
func (db *Database) deleteFilesFromAccount(userID uint) (err error) {
	return db.Model(&Files{}).
		Where(&Files{UploaderID: userID}).
		Delete(&Files{}).Error
}

func (db *Database) deleteSessionTokensFromAccount(userID uint) (err error) {
	return db.Model(&SessionTokens{}).
		Where(&SessionTokens{AccountID: userID}).
		Delete(&SessionTokens{}).Error
}

func (db *Database) deleteUploadTokensFromAccount(userID uint) (err error) {
	return db.Model(&UploadTokens{}).
		Where(&UploadTokens{AccountID: userID}).
		Delete(&UploadTokens{}).Error
}

func (db *Database) deleteSessionsFromAccount(accountID uint) (err error) {
	return db.Model(&SessionTokens{}).
		Where(&SessionTokens{AccountID: accountID}).
		Delete(&SessionTokens{}).Error
}

func (db *Database) deleteInviteCodesFromAccount(userID uint) (err error) {
	return db.Model(&InviteCodes{}).
		Where(&InviteCodes{InviteCreatorID: userID}).
		Delete(&InviteCodes{}).Error
}

// Deletes account entry only
func (db *Database) deleteAccount(userID uint) (err error) {
	return db.Model(&Accounts{}).
		Delete(&Accounts{}, userID).Error
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

func (db *Database) filesAmountOnAccount(accountID uint) (count int64, err error) {
	err = db.Model(&Files{}).
		Where(&Files{UploaderID: accountID}).
		Where("(expiry_date is not null AND expiry_date > ?) OR expiry_date is null", time.Now()). // Filters expired files
		Count(&count).Error

	return
}

func (db *Database) getAllFilesFromAccount(userID uint) (files []Files, err error) {
	err = db.Model(&Files{}).
		Where(&Files{UploaderID: userID}).
		Where("(expiry_date is not null AND expiry_date > ?) OR expiry_date is null", time.Now()). // Filters expired files
		Find(&files).Error

	return
}

func (db *Database) getAccountByID(accountID uint) (account Accounts, err error) {
	err = db.Model(&Accounts{}).
		Where(&Accounts{ID: accountID}).
		First(&account).Error

	return
}

func (db *Database) getSessionsCount(accountID uint) (count int64, err error) {
	err = db.Model(&SessionTokens{}).
		Where(&SessionTokens{AccountID: accountID}).
		Where("expiry_date > ?", time.Now()).
		Count(&count).Error

	return
}

func (db *Database) getUploadTokensCount(accountID uint) (count int64, err error) {
	err = db.Model(&UploadTokens{}).
		Where(&UploadTokens{AccountID: accountID}).
		Count(&count).Error

	return
}

// Looks if file exists in database
func (db *Database) fileExists(fileName string) (bool, error) {
	var count int64
	if err := db.Model(&Files{}).
		Where(&Files{FileName: fileName}).
		Where("(expiry_date is not null AND expiry_date > ?) OR expiry_date is null", time.Now()).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (db *Database) getFileByName(fileName string) (file Files, err error) {
	err = db.Model(&Files{}).
		Where(&Files{FileName: fileName}).
		Where("(expiry_date is not null AND expiry_date > ?) OR expiry_date is null", time.Now()).
		First(&file).Error

	return
}

func (db *Database) bumpFileViews(fileName string, ip string) (err error) {
	h := sha1.New()
	h.Write([]byte(ip))
	ipHash := hex.EncodeToString(h.Sum(nil))

	var fileID uint
	if err = db.Model(&Files{}).
		Where(&Files{FileName: fileName}).
		Select("id").
		Scan(&fileID).Error; err != nil {
		return
	}

	return db.Model(&FileViews{}).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&FileViews{
			IpHash:  ipHash,
			FilesID: fileID,
		}).Error
}

func (db *Database) createSessionToken(userID uint) (sessionToken uuid.UUID, err error) {
	log.Debug().Msgf("Creating session token for account %d", userID)

	session := SessionTokens{
		AccountID:  userID,
		Token:      uuid.New(),
		ExpiryDate: time.Now().Add(time.Hour * 24 * 7), // A week from now
		LastUsed:   time.Now(),
	}

	if err = db.Model(&SessionTokens{}).Create(&session).Error; err != nil {
		return
	}

	sessionToken = session.Token

	return
}

var ErrInvalidAccountType = errors.New("Invalid account type specified")

func (db *Database) createAccount(accountType string, invitedBy uint) (account Accounts, err error) {
	if accountType == "ADMIN" || accountType == "USER" {
		account = Accounts{
			AccountType: accountType,
			InvitedBy:   invitedBy,
		}

		err = db.Model(&Accounts{}).Create(&account).Error
	} else {
		err = ErrInvalidAccountType
	}

	return
}

type UiUploadToken struct {
	Token    uuid.UUID
	Nickname string
	LastUsed *time.Time
}

func (db *Database) getUploadTokens(userID uint) (uploadTokens []UiUploadToken, err error) {
	err = db.Model(&UploadTokens{}).
		Where(&UploadTokens{AccountID: userID}).
		Select("token, nickname, last_used").
		Scan(&uploadTokens).Error

	return
}

func (db *Database) createUploadToken(userID uint, nickname string) (uploadToken uuid.UUID, err error) {
	uploadToken = uuid.New()

	err = db.Model(&UploadTokens{}).
		Create(&UploadTokens{
			AccountID: userID,
			Token:     uploadToken,
			LastUsed:  nil,
			Nickname:  nickname,
		}).Error

	return
}

func (db *Database) deleteUploadToken(userID uint, uploadToken uuid.UUID) (err error) {
	return db.Model(&UploadTokens{}).
		Where(&UploadTokens{
			AccountID: userID,
			Token:     uploadToken,
		}).
		Delete(&UploadTokens{}).Error
}

func (db *Database) findExpiredFiles() (files []Files, err error) {
	err = db.Model(&Files{}).
		Where("expiry_date is not null AND expiry_date < ?", time.Now()).
		Find(&files).Error

	return
}

func (db *Database) deleteExpiredFiles() (err error) {
	return db.Model(&Files{}).
		Where("expiry_date is not null AND expiry_date < ?", time.Now()).
		Delete(&Files{}).Error
}

func (db *Database) deleteExpiredSessionTokens() (err error) {
	return db.Model(&SessionTokens{}).
		Where("expiry_date is not null AND expiry_date < ?", time.Now()).
		Delete(&SessionTokens{}).Error
}

func (db *Database) deleteExpiredInviteCodes() (err error) {
	return db.Model(&InviteCodes{}).
		Where("expiry_date is not null AND expiry_date < ?", time.Now()).
		Delete(&InviteCodes{}).Error
}

func (db *Database) getFilesPaginatedFromAccount(accountID, skip, limit uint, sort string, desc bool) (files []Files, err error) {
	if err = db.Model(&Files{}).
		Where("uploader_id = ?", accountID).
		Where("expiry_date IS NULL OR expiry_date > ?", time.Now()).
		Offset(int(skip)).
		Limit(int(limit)).
		Preload("Views").
		Joins("LEFT JOIN file_views ON file_views.files_id = files.id").
		Select("files.*, COUNT(file_views.id) AS views").
		Group("files.id").Order(clause.OrderByColumn{
		Column: clause.Column{Name: sort},
		Desc:   desc,
	}).Find(&files).Error; err != nil {
		return
	}

	for i, file := range files {
		files[i].ViewsCount = uint(len(file.Views))
	}

	return
}

func (db *Database) toggleFilePublic(fileName string, accountID uint) (newPublicStatus bool, err error) {
	var file Files

	if err = db.Model(&Files{}).
		Where(&Files{FileName: fileName, UploaderID: accountID}).
		Where("(expiry_date is not null AND expiry_date > ?) OR expiry_date is null", time.Now()).
		First(&file).Error; err != nil {
		return
	}

	newPublicStatus = !file.Public

	err = db.Model(&Files{}).
		Where(&Files{FileName: fileName, UploaderID: accountID}).
		Update("public", newPublicStatus).Error

	return
}
