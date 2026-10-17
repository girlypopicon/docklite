package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/backup"
	"docklite-agent/internal/docker"
	"docklite-agent/internal/store"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

const databaseRetentionCount = 7

func (h *Handlers) DatabaseStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	dockerUsers := make(map[string]string)
	dockerPasswords := make(map[string]string)
	if dockerDatabases, err := h.docker.ListDatabases(ctx); err == nil {
		for _, db := range dockerDatabases {
			if db.ID != "" && db.Username != "" {
				dockerUsers[db.ID] = db.Username
			}
			if db.ID != "" && db.Password != "" {
				dockerPasswords[db.ID] = db.Password
			}
			_, _ = h.store.UpsertDatabase(db.Name, db.Type, db.ID, db.Port, "")
		}
	}

	dbPath := h.store.Path
	if dbPath == "" {
		dbPath = filepath.Join("data", "docklite.db")
	}

	dockliteDbSize := int64(0)
	if stat, err := os.Stat(dbPath); err == nil {
		dockliteDbSize = stat.Size()
	}

	tableCount, err := h.store.CountTables()
	if err != nil {
		tableCount = 0
	}

	var records []store.DatabaseRecord
	if isAdminRole(r) {
		records, err = h.store.ListDatabases()
	} else {
		userID, _ := readUserID(r)
		if userID == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		records, err = h.store.ListDatabasesByUser(*userID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	withSize := make([]map[string]any, 0, len(records))
	for _, database := range records {
		size := int64(0)
		sizeCategory := "empty"

		if database.ContainerID != "" {
			if database.Type == "sqlite" {
				// TODO: Implement size check for SQLite file in container
				size = 0
				sizeCategory = "unknown"
			} else {
				query := fmt.Sprintf("SELECT pg_database_size('%s')", database.Name)
				dockerCtx, dockerCancel := dockerContext(r.Context())
				dbUser := dockerUsers[database.ContainerID]
				dbPassword := dockerPasswords[database.ContainerID]
				if dbPassword != "" && (dbUser != "" || database.Type == "redis") {
					var output string
					var err error
					if database.Type == "" || database.Type == "postgres" {
						output, err = h.docker.ExecPostgres(dockerCtx, database.ContainerID, dbUser, database.Name, dbPassword, query, false)
					} else {
						var n int64
						n, err = h.docker.EngineSize(dockerCtx, database.ContainerID, database.Type, database.Name, dbUser, dbPassword)
						output = strconv.FormatInt(n, 10)
					}
					if err == nil {
						if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(output), 10, 64); parseErr == nil {
							size = parsed
							switch {
							case size == 0:
								sizeCategory = "empty"
							case size < 1024*1024:
								sizeCategory = "tiny"
							case size < 10*1024*1024:
								sizeCategory = "small"
							case size < 100*1024*1024:
								sizeCategory = "medium"
							case size < 1024*1024*1024:
								sizeCategory = "large"
							default:
								sizeCategory = "huge"
							}
						}
					}
				}
				dockerCancel()
			}
		}

		withSize = append(withSize, map[string]any{
			"id":             database.ID,
			"name":           database.Name,
			"container_id":   database.ContainerID,
			"type":           databaseTypeOrDefault(database.Type),
			"postgres_port":  database.PostgresPort,
			"port":           database.PostgresPort,
			"container_host": docker.ContainerNameFor(database.Name),
			"created_at":     database.CreatedAt,
			"size":           size,
			"sizeCategory":   sizeCategory,
			"username":       dockerUsers[database.ContainerID],
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dockliteDb": map[string]any{
			"size":   dockliteDbSize,
			"tables": tableCount,
			"path":   dbPath,
		},
		"databases": withSize,
	})
}

func (h *Handlers) DatabaseRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/databases/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	dbID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || dbID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}

	if len(parts) == 1 || parts[1] == "" {
		h.Database(w, r, dbID)
		return
	}

	switch parts[1] {
	case "query", "schema", "table", "update-rows":
		if rec, err := h.store.GetDatabaseByID(dbID); err == nil && rec != nil && rec.Type != "" && rec.Type != "postgres" && rec.Type != "sqlite" {
			e, _ := docker.EngineFor(rec.Type)
			writeError(w, http.StatusBadRequest, "browsing and editing data isn't available for "+e.Label+" yet: connect with your own client using the connection details")
			return
		}
	}
	switch parts[1] {
	case "query":
		h.DatabaseQuery(w, r, dbID)
	case "schema":
		h.DatabaseSchema(w, r, dbID)
	case "table":
		h.DatabaseTable(w, r, dbID)
	case "update-rows":
		h.DatabaseUpdateRows(w, r, dbID)
	case "download":
		h.DatabaseDownload(w, r, dbID)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handlers) Database(w http.ResponseWriter, r *http.Request, databaseID int64) {
	switch r.Method {
	case http.MethodGet:
		h.getDatabase(w, r, databaseID)
	case http.MethodPatch:
		h.updateDatabaseCredentials(w, r, databaseID)
	case http.MethodDelete:
		h.deleteDatabase(w, r, databaseID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) getDatabase(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"database": database})
}

func (h *Handlers) deleteDatabase(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	if database.ContainerID != "" {
		dockerCtx, dockerCancel := dockerContext(r.Context())
		if err := h.docker.RemoveDatabaseContainer(dockerCtx, database.ContainerID, database.Name); err != nil {
			dockerCancel()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		dockerCancel()
	}

	if err := h.store.DeleteDatabase(databaseID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) updateDatabaseCredentials(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil || database.ContainerID == "" {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if database.Type != "" && database.Type != "postgres" {
		e, _ := docker.EngineFor(database.Type)
		writeError(w, http.StatusBadRequest, "changing the password isn't available for "+e.Label+" yet; delete and recreate it, or change it from your own client")
		return
	}

	dockerCtx, dockerCancel := dockerContext(r.Context())
	defer dockerCancel()

	adminUser := body.Username
	adminPassword := ""
	inspect, err := h.docker.InspectContainer(dockerCtx, database.ContainerID)
	if err != nil {
		log.Printf("update credentials failed to inspect container %s: %v", database.ContainerID, err)
	} else if inspect.Config != nil {
		if labelUser := inspect.Config.Labels["docklite.username"]; labelUser != "" {
			adminUser = labelUser
		}
		if labelPassword := inspect.Config.Labels["docklite.password"]; labelPassword != "" {
			adminPassword = labelPassword
		}
	}

	passwordSQL, err := formatSQLValue(body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid password")
		return
	}
	usernameSQL := quoteIdent(body.Username)
	databaseSQL := quoteIdent(database.Name)

	alterSQL := fmt.Sprintf("ALTER USER %s WITH PASSWORD %s;", usernameSQL, passwordSQL)
	if _, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, adminUser, database.Name, adminPassword, alterSQL, false); err != nil {
		log.Printf("update credentials failed to alter user %s: %v", body.Username, err)
		createSQL := fmt.Sprintf("CREATE USER %s WITH PASSWORD %s;", usernameSQL, passwordSQL)
		if _, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, adminUser, database.Name, adminPassword, createSQL, false); err != nil {
			log.Printf("update credentials failed to create user %s: %v", body.Username, err)
			writeError(w, http.StatusInternalServerError, "failed to update credentials")
			return
		}
		grantSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s;", databaseSQL, usernameSQL)
		if _, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, adminUser, database.Name, adminPassword, grantSQL, false); err != nil {
			log.Printf("update credentials failed to grant privileges for %s: %v", body.Username, err)
			writeError(w, http.StatusInternalServerError, "failed to grant privileges")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Database credentials updated successfully",
	})
}

func (h *Handlers) DatabaseQuery(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		SQL      string `json:"sql"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Username == "" || body.Password == "" || body.SQL == "" {
		writeError(w, http.StatusBadRequest, "username, password, and sql are required")
		return
	}

	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil || database.ContainerID == "" {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	dockerCtx, dockerCancel := dockerContext(r.Context())
	defer dockerCancel()

	normalized := normalizeSQL(body.SQL)
	isSelectable := strings.HasPrefix(strings.ToLower(normalized), "select") || strings.HasPrefix(strings.ToLower(normalized), "with")

	if isSelectable {
		wrapped := fmt.Sprintf("SELECT coalesce(json_agg(t), '[]'::json) FROM (%s) t;", normalized)
		rowsJSON, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, wrapped, true)
		if err != nil {
			if isAuthError(err) {
				writeError(w, http.StatusUnauthorized, "invalid database credentials")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to execute query")
			return
		}
		var rows []map[string]any
		if rowsJSON == "" {
			rowsJSON = "[]"
		}
		if err := json.Unmarshal([]byte(rowsJSON), &rows); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse query response")
			return
		}
		columns := columnsFromRows(rows)
		writeJSON(w, http.StatusOK, map[string]any{
			"type":    "select",
			"rows":    rows,
			"columns": columns,
		})
		return
	}

	output, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, normalized, false)
	if err != nil {
		if isAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid database credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to execute query")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"type":   "command",
		"output": output,
	})
}

func (h *Handlers) DatabaseSchema(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil || database.ContainerID == "" {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	tablesSQL := `
    SELECT coalesce(json_agg(t), '[]'::json)
    FROM (
      SELECT table_name
      FROM information_schema.tables
      WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
      ORDER BY table_name
    ) t;
  `

	columnsSQL := `
    SELECT coalesce(json_agg(t), '[]'::json)
    FROM (
      SELECT table_name, column_name, data_type, is_nullable, column_default
      FROM information_schema.columns
      WHERE table_schema = 'public'
      ORDER BY table_name, ordinal_position
    ) t;
  `

	keysSQL := `
    SELECT coalesce(json_agg(t), '[]'::json)
    FROM (
      SELECT kcu.table_name, kcu.column_name, tc.constraint_type
      FROM information_schema.table_constraints tc
      JOIN information_schema.key_column_usage kcu
        ON tc.constraint_name = kcu.constraint_name
        AND tc.table_schema = kcu.table_schema
        AND tc.table_name = kcu.table_name
      WHERE tc.table_schema = 'public'
        AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE', 'FOREIGN KEY')
      ORDER BY kcu.table_name, kcu.ordinal_position
    ) t;
  `

	dockerCtx, dockerCancel := dockerContext(r.Context())
	defer dockerCancel()

	tablesJSON, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, tablesSQL, true)
	if err != nil {
		if isAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid database credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch schema")
		return
	}
	columnsJSON, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, columnsSQL, true)
	if err != nil {
		if isAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid database credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch schema")
		return
	}
	keysJSON, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, keysSQL, true)
	if err != nil {
		if isAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid database credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch schema")
		return
	}

	var tables []map[string]any
	var columns []map[string]any
	var keys []map[string]any
	if tablesJSON == "" {
		tablesJSON = "[]"
	}
	if columnsJSON == "" {
		columnsJSON = "[]"
	}
	if keysJSON == "" {
		keysJSON = "[]"
	}
	_ = json.Unmarshal([]byte(tablesJSON), &tables)
	_ = json.Unmarshal([]byte(columnsJSON), &columns)
	_ = json.Unmarshal([]byte(keysJSON), &keys)

	keysByColumn := make(map[string][]string)
	for _, key := range keys {
		tableName, _ := key["table_name"].(string)
		columnName, _ := key["column_name"].(string)
		keyType, _ := key["constraint_type"].(string)
		if tableName == "" || columnName == "" || keyType == "" {
			continue
		}
		keyID := tableName + "." + columnName
		existing := keysByColumn[keyID]
		alreadyAdded := false
		for _, item := range existing {
			if item == keyType {
				alreadyAdded = true
				break
			}
		}
		if !alreadyAdded {
			keysByColumn[keyID] = append(existing, keyType)
		}
	}

	columnsByTable := make(map[string][]map[string]any)
	for _, col := range columns {
		tableName, _ := col["table_name"].(string)
		columnsByTable[tableName] = append(columnsByTable[tableName], col)
	}

	responseTables := make([]map[string]any, 0, len(tables))
	for _, table := range tables {
		tableName, _ := table["table_name"].(string)
		cols := columnsByTable[tableName]
		formatted := make([]map[string]any, 0, len(cols))
		for _, col := range cols {
			name, _ := col["column_name"].(string)
			colType, _ := col["data_type"].(string)
			nullable := false
			if nullableRaw, ok := col["is_nullable"].(string); ok && nullableRaw == "YES" {
				nullable = true
			}
			keyID := tableName + "." + name
			keyType := ""
			if keyValues := keysByColumn[keyID]; len(keyValues) > 0 {
				keyType = strings.Join(keyValues, ", ")
			}
			formatted = append(formatted, map[string]any{
				"name":     name,
				"type":     colType,
				"nullable": nullable,
				"default":  col["column_default"],
				"key":      keyType,
			})
		}
		responseTables = append(responseTables, map[string]any{
			"name":    tableName,
			"columns": formatted,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"tables": responseTables})
}

func (h *Handlers) DatabaseTable(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Table    string `json:"table"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Username == "" || body.Password == "" || body.Table == "" {
		writeError(w, http.StatusBadRequest, "username, password, and table are required")
		return
	}
	if !identifierPattern.MatchString(body.Table) {
		writeError(w, http.StatusBadRequest, "invalid table name")
		return
	}

	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil || database.ContainerID == "" {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	columnsSQL := fmt.Sprintf(`
    SELECT coalesce(json_agg(t), '[]'::json)
    FROM (
      SELECT column_name, data_type, is_nullable
      FROM information_schema.columns
      WHERE table_schema = 'public' AND table_name = '%s'
      ORDER BY ordinal_position
    ) t;
  `, body.Table)

	rowsSQL := fmt.Sprintf(`
    SELECT coalesce(json_agg(t), '[]'::json)
    FROM (
      SELECT * FROM "%s" LIMIT 100
    ) t;
  `, body.Table)

	dockerCtx, dockerCancel := dockerContext(r.Context())
	defer dockerCancel()

	columnsJSON, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, columnsSQL, true)
	if err != nil {
		if isAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid database credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch table data")
		return
	}
	rowsJSON, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, rowsSQL, true)
	if err != nil {
		if isAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid database credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch table data")
		return
	}

	var columns []map[string]any
	var rows []map[string]any
	if columnsJSON == "" {
		columnsJSON = "[]"
	}
	if rowsJSON == "" {
		rowsJSON = "[]"
	}
	_ = json.Unmarshal([]byte(columnsJSON), &columns)
	_ = json.Unmarshal([]byte(rowsJSON), &rows)

	formattedColumns := make([]map[string]any, 0, len(columns))
	for _, col := range columns {
		name, _ := col["column_name"].(string)
		colType, _ := col["data_type"].(string)
		nullable := false
		if nullableRaw, ok := col["is_nullable"].(string); ok && nullableRaw == "YES" {
			nullable = true
		}
		formattedColumns = append(formattedColumns, map[string]any{
			"name":     name,
			"type":     colType,
			"nullable": nullable,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"columns": formattedColumns,
		"rows":    rows,
	})
}

func (h *Handlers) DatabaseUpdateRows(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Username string           `json:"username"`
		Password string           `json:"password"`
		Table    string           `json:"table"`
		Rows     []map[string]any `json:"rows"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Username == "" || body.Password == "" || body.Table == "" {
		writeError(w, http.StatusBadRequest, "username, password, and table are required")
		return
	}
	if !identifierPattern.MatchString(body.Table) {
		writeError(w, http.StatusBadRequest, "invalid table name")
		return
	}
	if len(body.Rows) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}

	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil || database.ContainerID == "" {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	dockerCtx, dockerCancel := dockerContext(r.Context())
	defer dockerCancel()

	for _, row := range body.Rows {
		rowID, ok := row["id"]
		if !ok {
			writeError(w, http.StatusBadRequest, "each row must include an id field")
			return
		}

		idValue, err := formatSQLValue(rowID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid row id")
			return
		}

		setClauses := make([]string, 0)
		for column, value := range row {
			if column == "id" {
				continue
			}
			if !identifierPattern.MatchString(column) {
				writeError(w, http.StatusBadRequest, "invalid column name")
				return
			}
			valueSQL, err := formatSQLValue(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid column value")
				return
			}
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdent(column), valueSQL))
		}

		if len(setClauses) == 0 {
			continue
		}

		updateSQL := fmt.Sprintf(
			"UPDATE %s SET %s WHERE %s = %s;",
			quoteIdent(body.Table),
			strings.Join(setClauses, ", "),
			quoteIdent("id"),
			idValue,
		)

		if _, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, updateSQL, false); err != nil {
			if isAuthError(err) {
				writeError(w, http.StatusUnauthorized, "invalid database credentials")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to update rows")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handlers) DatabaseDownload(w http.ResponseWriter, r *http.Request, databaseID int64) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !isAdminRole(r) {
		userID, err := readUserID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user id")
			return
		}
		if userID == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		allowed, err := h.store.HasDatabaseAccess(*userID, databaseID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Gzip     *bool  `json:"gzip"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	database, err := h.store.GetDatabaseByID(databaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if database == nil || database.ContainerID == "" {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	// Redis has a password but no user name.
	if body.Password == "" || (body.Username == "" && database.Type != "redis") {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	dockerCtx, dockerCancel := dockerContext(r.Context())
	defer dockerCancel()

	engine := databaseTypeOrDefault(database.Type)
	if engine == "postgres" {
		if _, err := h.docker.ExecPostgres(dockerCtx, database.ContainerID, body.Username, database.Name, body.Password, "SELECT 1", false); err != nil {
			writeError(w, http.StatusBadRequest, "invalid database credentials")
			return
		}
	} else if _, err := h.docker.EngineSize(dockerCtx, database.ContainerID, engine, database.Name, body.Username, body.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database credentials")
		return
	}

	notes := "download"
	source := backup.ManifestSource{
		DatabaseID:  &database.ID,
		Name:        database.Name,
		ContainerID: database.ContainerID,
		Engine:      engine,
	}
	artifact, err := backup.CreateDatabaseExportOfType(dockerCtx, h.docker, engine, h.backupBaseDir, "databases", database.Name, database.ContainerID, body.Username, &body.Password, source, notes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create database dump")
		return
	}

	info, err := os.Stat(artifact.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	file, err := os.Open(artifact.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(artifact.Path)+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)

	pruneDatabaseDumps(h.backupBaseDir, database.Name)
}

func pruneDatabaseDumps(baseDir string, dbName string) {
	root := filepath.Join(baseDir, "databases")
	entries, err := os.ReadDir(root)
	if err != nil {
		log.Printf("prune database dumps failed: %v", err)
		return
	}

	prefix := fmt.Sprintf("db-%s-", backup.SanitizeFilename(dbName))
	type dumpFile struct {
		path    string
		modTime time.Time
		base    string
	}

	var dumps []dumpFile
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if !strings.HasSuffix(name, ".dump") && !strings.HasSuffix(name, ".dump.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(name, ".gz")
		base = strings.TrimSuffix(base, ".dump")
		dumps = append(dumps, dumpFile{
			path:    filepath.Join(root, name),
			modTime: info.ModTime(),
			base:    base,
		})
	}

	sort.Slice(dumps, func(i, j int) bool {
		return dumps[i].modTime.After(dumps[j].modTime)
	})

	for i := databaseRetentionCount; i < len(dumps); i++ {
		_ = os.Remove(dumps[i].path)
		_ = os.Remove(filepath.Join(root, fmt.Sprintf("%s.json", dumps[i].base)))
		_ = os.Remove(backup.ManifestPathForArtifact(dumps[i].path))
	}
}

func normalizeSQL(sql string) string {
	trimmed := strings.TrimSpace(sql)
	return strings.TrimRightFunc(trimmed, func(r rune) bool {
		return r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
}

func columnsFromRows(rows []map[string]any) []string {
	if len(rows) == 0 {
		return []string{}
	}
	columns := make([]string, 0, len(rows[0]))
	for key := range rows[0] {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	return columns
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "password authentication failed") {
		return true
	}
	if strings.Contains(message, "no password supplied") {
		return true
	}
	if strings.Contains(message, "authentication failed") {
		return true
	}
	if strings.Contains(message, "role") && strings.Contains(message, "does not exist") {
		return true
	}
	return false
}

func formatSQLValue(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "NULL", nil
	case string:
		escaped := strings.ReplaceAll(typed, "'", "''")
		return "'" + escaped + "'", nil
	case bool:
		if typed {
			return "TRUE", nil
		}
		return "FALSE", nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", fmt.Errorf("invalid number")
		}
		if typed == math.Trunc(typed) {
			return strconv.FormatInt(int64(typed), 10), nil
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case float32:
		value64 := float64(typed)
		if math.IsNaN(value64) || math.IsInf(value64, 0) {
			return "", fmt.Errorf("invalid number")
		}
		if value64 == math.Trunc(value64) {
			return strconv.FormatInt(int64(value64), 10), nil
		}
		return strconv.FormatFloat(value64, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	default:
		serialized, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		escaped := strings.ReplaceAll(string(serialized), "'", "''")
		return "'" + escaped + "'", nil
	}
}

func databaseTypeOrDefault(t string) string {
	if t == "" {
		return "postgres"
	}
	return t
}
