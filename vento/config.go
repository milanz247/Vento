package vento

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadEnv reads a simple KEY=VALUE .env file (blank lines and lines
// starting with # are ignored; surrounding quotes on the value are
// stripped). Every pair is also exported into the process environment via
// os.Setenv, so application code can read config through either the
// returned map or the standard os.Getenv. A missing file is not an error:
// it simply yields an empty map, letting callers rely on already-exported
// environment variables instead.
func LoadEnv(filePath string) map[string]string {
	env := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return env
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		env[key] = value
		os.Setenv(key, value)
	}
	_ = scanner.Err() // best-effort parse; a mid-file read error just stops early

	return env
}

// BuildMySQLDSN assembles the DSN string gorm.io/driver/mysql expects out
// of the discrete DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME entries in
// env (as returned by LoadEnv), so applications never hand-build or store
// a raw DSN. It reports ok=false when DB_HOST, DB_USER, or DB_NAME is
// missing, letting the caller abort startup loudly instead of attempting
// a connection that cannot succeed. DB_PORT defaults to 3306.
func BuildMySQLDSN(env map[string]string) (dsn string, ok bool) {
	host := env["DB_HOST"]
	user := env["DB_USER"]
	name := env["DB_NAME"]
	if host == "" || user == "" || name == "" {
		return "", false
	}

	port := env["DB_PORT"]
	if port == "" {
		port = "3306"
	}

	dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, env["DB_PASSWORD"], host, port, name)
	return dsn, true
}
