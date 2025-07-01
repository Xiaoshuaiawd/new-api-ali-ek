package common

const (
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeSQLite     = "sqlite"
	DatabaseTypePostgreSQL = "postgres"
)

var UsingSQLite = false
var UsingPostgreSQL = false
var LogSqlType = DatabaseTypeSQLite // Default to SQLite for logging SQL queries
var UsingMySQL = false
var UsingClickHouse = false

// MES数据库相关变量 - 用于聊天历史存储
var UsingMESMySQL = false
var UsingMESPostgreSQL = false
var UsingMESSQLite = false
var MESSqlType = DatabaseTypeSQLite // Default to SQLite for MES (conversation history) database

var SQLitePath = "one-api.db?_busy_timeout=5000"
