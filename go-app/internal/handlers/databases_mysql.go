package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"docklite-agent/internal/store"
)

// MariaDB and MySQL are browsed the way Postgres is: by running the database's own command line client inside its
// container (so no extra driver and no published port), with the answers in the same shapes the dashboard's table
// editor already reads. The client prints query results as XML, which tells an empty value from NULL.

type mysqlField struct {
	Name  string `xml:"name,attr"`
	Nil   string `xml:"nil,attr"`
	Value string `xml:",chardata"`
}

type mysqlResult struct {
	Rows []struct {
		Fields []mysqlField `xml:"field"`
	} `xml:"row"`
}

// parseMySQLXML turns the client's --xml output into column names and rows (NULL becomes null).
func parseMySQLXML(out string) ([]string, []map[string]any, error) {
	out = strings.TrimSpace(out)
	columns := []string{}
	rows := []map[string]any{}
	if out == "" {
		return columns, rows, nil
	}
	var res mysqlResult
	if err := xml.Unmarshal([]byte(out), &res); err != nil {
		return nil, nil, fmt.Errorf("could not read the database's answer: %v", err)
	}
	for i, r := range res.Rows {
		row := make(map[string]any, len(r.Fields))
		for _, f := range r.Fields {
			if i == 0 {
				columns = append(columns, f.Name)
			}
			if f.Nil == "true" {
				row[f.Name] = nil
			} else {
				row[f.Name] = f.Value
			}
		}
		rows = append(rows, row)
	}
	return columns, rows, nil
}

func mysqlQuoteIdent(name string) string { return "`" + strings.ReplaceAll(name, "`", "``") + "`" }

// mysqlLiteral writes a value from the editor as SQL text: null, a number or a quoted string.
func mysqlLiteral(v any) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	case float64:
		return fmt.Sprintf("%v", t)
	case string:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(t, `\`, `\\`), "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(fmt.Sprint(t), `\`, `\\`), "'", "''") + "'"
	}
}

// mysqlCleanError keeps the useful line of the client's complaint ("ERROR 1064 ... near ...") and drops the warnings
// and the dashed echo of the statement around it.
func mysqlCleanError(err error) string {
	for _, line := range strings.Split(err.Error(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "ERROR") {
			return strings.TrimSpace(line)
		}
	}
	return err.Error()
}

func mysqlClientName(dbType string) string {
	if dbType == "mariadb" {
		return "mariadb"
	}
	return "mysql"
}

func mysqlIsAuthError(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "access denied") || strings.Contains(m, "error 1045")
}

// mysqlExec runs one statement as the database's user and returns what the client printed.
func (h *Handlers) mysqlExec(rec *store.DatabaseRecord, ctx context.Context, user, pass, sql string) (string, error) {
	cmd := []string{
		mysqlClientName(rec.Type),
		"-u", user, "-h", "127.0.0.1", "-P", "3306",
		"--xml", "--default-character-set=utf8mb4",
		"-D", rec.Name,
		"-e", sql,
	}
	var out bytes.Buffer
	if err := h.docker.ExecCommandToWriter(ctx, rec.ContainerID, cmd, []string{"MYSQL_PWD=" + pass}, &out); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (h *Handlers) mysqlRows(rec *store.DatabaseRecord, ctx context.Context, user, pass, sql string) ([]string, []map[string]any, error) {
	out, err := h.mysqlExec(rec, ctx, user, pass, sql)
	if err != nil {
		return nil, nil, err
	}
	return parseMySQLXML(out)
}

// mysqlSchema lists the tables with their columns, like the Postgres answer.
func (h *Handlers) mysqlSchema(rec *store.DatabaseRecord, ctx context.Context, user, pass string) ([]map[string]any, error) {
	_, rows, err := h.mysqlRows(rec, ctx, user, pass, `SELECT c.table_name AS table_name, c.column_name AS column_name, c.column_type AS column_type,
 c.is_nullable AS is_nullable, c.column_default AS column_default, c.column_key AS column_key
 FROM information_schema.columns c JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
 WHERE c.table_schema = DATABASE() AND t.table_type = 'BASE TABLE' ORDER BY c.table_name, c.ordinal_position`)
	if err != nil {
		return nil, err
	}
	var tables []map[string]any
	index := map[string]int{}
	for _, r := range rows {
		name := fmt.Sprint(r["table_name"])
		i, ok := index[name]
		if !ok {
			index[name] = len(tables)
			i = len(tables)
			tables = append(tables, map[string]any{"name": name, "columns": []map[string]any{}})
		}
		key := ""
		if fmt.Sprint(r["column_key"]) == "PRI" {
			key = "PRIMARY KEY"
		}
		dflt := r["column_default"]
		if dflt == "NULL" { // MariaDB spells "no default" as the text NULL
			dflt = nil
		}
		col := map[string]any{
			"name":     fmt.Sprint(r["column_name"]),
			"type":     fmt.Sprint(r["column_type"]),
			"nullable": fmt.Sprint(r["is_nullable"]) == "YES",
			"default":  dflt,
			"key":      key,
		}
		tables[i]["columns"] = append(tables[i]["columns"].([]map[string]any), col)
	}
	if tables == nil {
		tables = []map[string]any{}
	}
	return tables, nil
}

// mysqlRoute handles /api/databases/{id}/{query,schema,table,update-rows} for MariaDB and MySQL.
func (h *Handlers) mysqlRoute(w http.ResponseWriter, r *http.Request, rec *store.DatabaseRecord, action string) {
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
		SQL      string           `json:"sql"`
		Rows     []map[string]any `json:"rows"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if rec.ContainerID == "" {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	fail := func(err error) {
		if mysqlIsAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid database credentials")
			return
		}
		writeError(w, http.StatusBadRequest, mysqlCleanError(err))
	}

	switch action {
	case "schema":
		tables, err := h.mysqlSchema(rec, ctx, body.Username, body.Password)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
	case "table":
		if !identifierPattern.MatchString(body.Table) {
			writeError(w, http.StatusBadRequest, "invalid table name")
			return
		}
		tables, err := h.mysqlSchema(rec, ctx, body.Username, body.Password)
		if err != nil {
			fail(err)
			return
		}
		var cols any
		for _, t := range tables {
			if t["name"] == body.Table {
				cols = t["columns"]
			}
		}
		if cols == nil {
			writeError(w, http.StatusBadRequest, "there is no table called "+body.Table)
			return
		}
		_, rows, err := h.mysqlRows(rec, ctx, body.Username, body.Password, "SELECT * FROM "+mysqlQuoteIdent(body.Table)+" LIMIT 100")
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "rows": rows})
	case "query":
		stmt := normalizeSQL(body.SQL)
		if stmt == "" {
			writeError(w, http.StatusBadRequest, "sql is required")
			return
		}
		lower := strings.ToLower(stmt)
		for _, p := range []string{"select", "with", "show", "describe", "desc ", "explain"} {
			if strings.HasPrefix(lower, p) {
				columns, rows, err := h.mysqlRows(rec, ctx, body.Username, body.Password, stmt)
				if err != nil {
					fail(err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"type": "select", "rows": rows, "columns": columns})
				return
			}
		}
		if _, err := h.mysqlExec(rec, ctx, body.Username, body.Password, stmt); err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"type": "command", "output": "OK"})
	case "update-rows":
		if !identifierPattern.MatchString(body.Table) {
			writeError(w, http.StatusBadRequest, "invalid table name")
			return
		}
		var stmts []string
		for _, row := range body.Rows {
			id, ok := row["id"]
			if !ok {
				writeError(w, http.StatusBadRequest, "each row must include an id field")
				return
			}
			var sets []string
			for col, val := range row {
				if col == "id" {
					continue
				}
				if !identifierPattern.MatchString(col) {
					writeError(w, http.StatusBadRequest, "invalid column name")
					return
				}
				sets = append(sets, mysqlQuoteIdent(col)+" = "+mysqlLiteral(val))
			}
			if len(sets) > 0 {
				stmts = append(stmts, "UPDATE "+mysqlQuoteIdent(body.Table)+" SET "+strings.Join(sets, ", ")+" WHERE `id` = "+mysqlLiteral(id)+";")
			}
		}
		if len(stmts) > 0 {
			if _, err := h.mysqlExec(rec, ctx, body.Username, body.Password, strings.Join(stmts, " ")); err != nil {
				fail(err)
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
