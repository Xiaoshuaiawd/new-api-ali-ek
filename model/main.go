package model

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"one-api/common"
	"one-api/constant"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string

func initCol() {
	// init common column names
	if common.UsingPostgreSQL {
		commonGroupCol = `"group"`
		commonKeyCol = `"key"`
		commonTrueVal = "true"
		commonFalseVal = "false"
	} else {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch common.LogSqlType {
		case common.DatabaseTypePostgreSQL:
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		default:
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	} else {
		// LOG_SQL_DSN 为空时，日志数据库与主数据库相同
		if common.UsingPostgreSQL {
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		} else {
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	}
	// log sql type and database type
	common.SysLog("Using Log SQL Type: " + common.LogSqlType)
}

var DB *gorm.DB

var LOG_DB *gorm.DB

var MES_DB *gorm.DB

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CheckSetup() {
	setup := GetSetup()
	if setup == nil {
		// No setup record exists, check if we have a root user
		if RootUserExists() {
			common.SysLog("system is not initialized, but root user exists")
			// Create setup record
			newSetup := Setup{
				Version:       common.Version,
				InitializedAt: time.Now().Unix(),
			}
			err := DB.Create(&newSetup).Error
			if err != nil {
				common.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			common.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
	} else {
		// Setup record exists, system is initialized
		common.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
		constant.Setup = true
	}
}

// autoCreateDatabase 自动创建数据库（如果不存在）
func autoCreateDatabase(dsn string) error {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return autoCreatePostgreSQLDatabase(dsn)
	}

	// 对于MySQL
	if strings.Contains(dsn, "@tcp(") {
		return autoCreateMySQLDatabase(dsn)
	}

	// SQLite不需要自动创建数据库
	return nil
}

// autoCreateMySQLDatabase 自动创建MySQL数据库
func autoCreateMySQLDatabase(dsn string) error {
	// 解析DSN，提取数据库名
	// 格式：username:password@tcp(host:port)/database?param1=value1
	parts := strings.Split(dsn, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid MySQL DSN format")
	}

	dbPart := parts[len(parts)-1]
	var dbName string
	if strings.Contains(dbPart, "?") {
		dbName = strings.Split(dbPart, "?")[0]
	} else {
		dbName = dbPart
	}

	if dbName == "" {
		return fmt.Errorf("database name not found in DSN")
	}

	// 创建不包含数据库名的DSN
	dsnWithoutDB := strings.Replace(dsn, "/"+dbPart, "/", 1)

	// 连接到MySQL服务器（不指定数据库）
	db, err := sql.Open("mysql", dsnWithoutDB)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL server: %v", err)
	}
	defer db.Close()

	// 检查数据库是否存在
	var exists int
	err = db.QueryRow("SELECT COUNT(*) FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check database existence: %v", err)
	}

	if exists == 0 {
		// 创建数据库
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName))
		if err != nil {
			return fmt.Errorf("failed to create database %s: %v", dbName, err)
		}
		common.SysLog(fmt.Sprintf("Database '%s' created successfully", dbName))
	}

	return nil
}

// autoCreatePostgreSQLDatabase 自动创建PostgreSQL数据库
func autoCreatePostgreSQLDatabase(dsn string) error {
	// 对于PostgreSQL，我们尝试使用GORM直接连接
	// 如果数据库不存在，让用户手动创建，因为PostgreSQL的权限管理比较复杂
	common.SysLog("PostgreSQL database auto-creation is not supported. Please ensure the database exists before starting the application.")

	// 解析DSN来获取数据库名
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse PostgreSQL DSN: %v", err)
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return fmt.Errorf("database name not found in PostgreSQL DSN")
	}

	common.SysLog(fmt.Sprintf("Please ensure PostgreSQL database '%s' exists before starting the application", dbName))
	return nil
}

func chooseDB(envName string, dbType string) (*gorm.DB, error) {
	defer func() {
		initCol()
	}()
	dsn := os.Getenv(envName)
	if dsn != "" {
		// 尝试自动创建数据库（如果不存在）
		if err := autoCreateDatabase(dsn); err != nil {
			common.SysLog(fmt.Sprintf("Failed to auto-create database: %v", err))
		}
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			common.SysLog("using PostgreSQL as database")
			if dbType == "main" {
				common.UsingPostgreSQL = true
			} else if dbType == "log" {
				common.LogSqlType = common.DatabaseTypePostgreSQL
			} else if dbType == "mes" {
				common.UsingMESPostgreSQL = true
				common.MESSqlType = common.DatabaseTypePostgreSQL
			}
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		if strings.HasPrefix(dsn, "local") {
			common.SysLog("SQL_DSN not set, using SQLite as database")
			if dbType == "main" {
				common.UsingSQLite = true
			} else if dbType == "log" {
				common.LogSqlType = common.DatabaseTypeSQLite
			} else if dbType == "mes" {
				common.UsingMESSQLite = true
				common.MESSqlType = common.DatabaseTypeSQLite
			}
			return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		// Use MySQL
		common.SysLog("using MySQL as database")
		// check parseTime
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if dbType == "main" {
			common.UsingMySQL = true
		} else if dbType == "log" {
			common.LogSqlType = common.DatabaseTypeMySQL
		} else if dbType == "mes" {
			common.UsingMESMySQL = true
			common.MESSqlType = common.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	common.SysLog("SQL_DSN not set, using SQLite as database")
	if dbType == "main" {
		common.UsingSQLite = true
	} else if dbType == "log" {
		common.LogSqlType = common.DatabaseTypeSQLite
	} else if dbType == "mes" {
		common.UsingMESSQLite = true
		common.MESSqlType = common.DatabaseTypeSQLite
	}
	return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB("SQL_DSN", "main")
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		if common.UsingMySQL {
			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		return
	}
	db, err := chooseDB("LOG_SQL_DSN", "log")
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		LOG_DB = db
		sqlDB, err := LOG_DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		//if common.UsingMySQL {
		//	_, _ = sqlDB.Exec("DROP INDEX idx_channels_key ON channels;")             // TODO: delete this line when most users have upgraded
		//	_, _ = sqlDB.Exec("ALTER TABLE midjourneys MODIFY action VARCHAR(40);")   // TODO: delete this line when most users have upgraded
		//	_, _ = sqlDB.Exec("ALTER TABLE midjourneys MODIFY progress VARCHAR(30);") // TODO: delete this line when most users have upgraded
		//	_, _ = sqlDB.Exec("ALTER TABLE midjourneys MODIFY status VARCHAR(20);")   // TODO: delete this line when most users have upgraded
		//}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitMESDB() (err error) {
	if os.Getenv("MES_SQL_DSN") == "" {
		MES_DB = DB
		common.SysLog("MES_SQL_DSN not set, using main database for conversation history")
		return
	}
	db, err := chooseDB("MES_SQL_DSN", "mes")
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		MES_DB = db
		sqlDB, err := MES_DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("MES database migration started")
		err = migrateMESDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func migrateDB() error {
	if !common.UsingPostgreSQL {
		return migrateDBFast()
	}
	err := DB.AutoMigrate(
		&Channel{},
		&Token{},
		&User{},
		&Option{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Midjourney{},
		&TopUp{},
		&QuotaData{},
		&Task{},
		&Setup{},
	)
	if err != nil {
		return err
	}
	return nil
}

func migrateDBFast() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 12) // Buffer size matches number of migrations

	migrations := []struct {
		model interface{}
		name  string
	}{
		{&Channel{}, "Channel"},
		{&Token{}, "Token"},
		{&User{}, "User"},
		{&Option{}, "Option"},
		{&Redemption{}, "Redemption"},
		{&Ability{}, "Ability"},
		{&Log{}, "Log"},
		{&Midjourney{}, "Midjourney"},
		{&TopUp{}, "TopUp"},
		{&QuotaData{}, "QuotaData"},
		{&Task{}, "Task"},
		{&Setup{}, "Setup"},
	}

	for _, m := range migrations {
		wg.Add(1)
		go func(model interface{}, name string) {
			defer wg.Done()
			if err := DB.AutoMigrate(model); err != nil {
				errChan <- fmt.Errorf("failed to migrate %s: %v", name, err)
			}
		}(m.model, m.name)
	}

	// Wait for all migrations to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	common.SysLog("database migrated")
	return nil
}

func migrateLOGDB() error {
	var err error
	if err = LOG_DB.AutoMigrate(&Log{}); err != nil {
		return err
	}
	return nil
}

func migrateMESDB() error {
	var err error
	if err = MES_DB.AutoMigrate(
		&ConversationHistory{},
		&ErrorConversationHistory{},
	); err != nil {
		return err
	}
	common.SysLog("MES database migrated")
	return nil
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	// 关闭 LOG_DB（如果与主数据库不同）
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}

	// 关闭 MES_DB（如果与主数据库不同）
	if MES_DB != DB && MES_DB != nil {
		err := closeDB(MES_DB)
		if err != nil {
			return err
		}
	}

	// 关闭主数据库
	return closeDB(DB)
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
