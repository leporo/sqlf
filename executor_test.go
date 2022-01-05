package sqlf_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/leporo/sqlf"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

type dbEnv struct {
	driver string
	db     *sql.DB
	sqlf   *sqlf.Dialect
}

type dbConfig struct {
	driver  string
	envVar  string
	defDSN  string
	dialect *sqlf.Dialect
}

var dbList = []dbConfig{
	{
		driver:  "sqlite3",
		envVar:  "SQLF_SQLITE_DSN",
		defDSN:  ":memory:",
		dialect: sqlf.NoDialect,
	},
}

var envs = make([]dbEnv, 0, len(dbList))

func init() {
	connect()
}

func connect() {
	// Connect to databases
	for _, config := range dbList {
		dsn := os.Getenv(config.envVar)
		if dsn == "" {
			dsn = config.defDSN
		}
		if dsn == "" || dsn == "skip" {
			fmt.Printf("Skipping %s tests.", config.driver)
			continue
		}
		db, err := sql.Open(config.driver, dsn)
		if err != nil {
			log.Fatalf("Invalid %s DSN: %v", config.driver, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err != nil {
			log.Fatalf("Unable to connect to %s: %v", config.driver, err)
		}
		envs = append(envs, dbEnv{
			driver: config.driver,
			db:     db,
			sqlf:   config.dialect,
		})
	}
}

func execScript(db *sql.DB, script []string) (err error) {
	for _, stmt := range script {
		_, err = db.Exec(stmt)
		if err != nil {
			break
		}
	}
	return err
}

func forEveryDB(t *testing.T, test func(ctx context.Context, env *dbEnv)) {
	for _, ctx := range []context.Context{nil, context.Background()} {
		for n := range envs {
			env := &envs[n]
			// Create schema
			err := execScript(env.db, sqlSchemaCreate)
			if err != nil {
				t.Errorf("Failed to create a %s schema: %v", env.driver, err)
			} else {
				err = execScript(env.db, sqlFillDb)
				if err != nil {
					t.Errorf("Failed to populate a %s database: %v", env.driver, err)
				} else {
					// Execute a test
					test(ctx, env)
				}
			}
			err = execScript(env.db, sqlSchemaDrop)
			if err != nil {
				t.Errorf("Failed to drop a %s schema: %v", env.driver, err)
			}
		}
	}
}

func TestQueryRow(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		var name string
		q := env.sqlf.From("users").
			Select("name").To(&name).
			Where("id = ?", 1)
		err := q.QueryRow(ctx, env.db)
		q.Close()
		require.NoError(t, err, "Failed to execute a query: %v", err)
		require.Equal(t, "User 1", name)
	})
}

func TestQueryRowAndClose(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		var name string
		err := env.sqlf.From("users").
			Select("name").To(&name).
			Where("id = ?", 1).
			QueryRowAndClose(ctx, env.db)
		require.NoError(t, err, "Failed to execute a query: %v", err)
		require.Equal(t, "User 1", name)
	})
}

func TestBind(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		var u struct {
			ID   int64  `db:"id"`
			Name string `db:"name"`
		}
		err := env.sqlf.From("users").
			Bind(&u).
			Where("id = ?", 2).
			QueryRowAndClose(ctx, env.db)
		require.NoError(t, err, "Failed to execute a query: %v", err)
		require.Equal(t, "User 2", u.Name)
		require.EqualValues(t, 2, u.ID)
	})
}

func TestBindNested(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		type Parent struct {
			ID int64 `db:"id"`
		}
		var u struct {
			Parent
			Name string `db:"name"`
		}
		err := env.sqlf.From("users").
			Bind(&u).
			Where("id = ?", 2).
			QueryRowAndClose(ctx, env.db)
		require.NoError(t, err, "Failed to execute a query: %v", err)
		require.Equal(t, "User 2", u.Name)
		require.EqualValues(t, 2, u.ID)
	})
}

func TestExec(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		var (
			userId int
			count  int
		)
		q := env.sqlf.From("users").
			Select("count(*)").To(&count).
			Select("min(id)").To(&userId)
		defer q.Close()

		q.QueryRow(ctx, env.db)

		require.Equal(t, 3, count)

		_, err := env.sqlf.DeleteFrom("users").
			Where("id = ?", userId).
			ExecAndClose(ctx, env.db)
		require.NoError(t, err, "Failed to delete a row. %s error: %v", env.driver, err)

		// Re-check the number of remaining rows
		count = 0
		q.QueryRow(ctx, env.db)

		require.Equal(t, 2, count)
	})
}

func TestPagination(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		type Income struct {
			Id         int64   `db:"id"`
			UserId     int64   `db:"user_id"`
			FromUserId int64   `db:"from_user_id"`
			Amount     float64 `db:"amount"`
		}

		type PaginatedIncomes struct {
			Count int64
			Data  []Income
		}

		var (
			result PaginatedIncomes
			o      Income
			err    error
		)

		// Create a base query, apply filters
		qs := sqlf.From("incomes").Where("amount > ?", 100)
		// Clone a statement and retrieve the record count
		err = qs.Clone().
			Select("count(id)").To(&result.Count).
			QueryRowAndClose(ctx, env.db)
		if err != nil {
			return
		}

		// Retrieve page data
		err = qs.Bind(&o).
			OrderBy("id desc").
			Paginate(1, 2).
			QueryAndClose(ctx, env.db, func(rows *sql.Rows) {
				result.Data = append(result.Data, o)
			})
		if err != nil {
			return
		}
		require.EqualValues(t, 4, result.Count)
		require.Len(t, result.Data, 2)
	})
}

func TestQuery(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		var (
			nRows    int = 0
			userTo   string
			userFrom string
			amount   float64
		)
		q := env.sqlf.
			From("incomes").
			From("users ut").Where("ut.id = user_id").
			From("users uf").Where("uf.id = from_user_id").
			Select("ut.name").To(&userTo).
			Select("uf.name").To(&userFrom).
			Select("sum(amount) as got").To(&amount).
			GroupBy("ut.name, uf.name").
			OrderBy("got DESC")
		defer q.Close()
		err := q.Query(ctx, env.db, func(rows *sql.Rows) {
			nRows++
		})
		if err != nil {
			t.Errorf("Failed to execute a query: %v", err)
		} else {
			require.Equal(t, 4, nRows)

			q.Limit(1)

			nRows = 0
			err := q.Query(ctx, env.db, func(rows *sql.Rows) {
				nRows++
			})
			if err != nil {
				t.Errorf("Failed to execute a query: %v", err)
			} else {
				require.Equal(t, 1, nRows)
				require.Equal(t, "User 3", userTo)
				require.Equal(t, "User 1", userFrom)
				require.Equal(t, 500.0, amount)
			}
		}
	})
}

func TestQueryAndClose(t *testing.T) {
	forEveryDB(t, func(ctx context.Context, env *dbEnv) {
		var (
			nRows  int     = 0
			total  float64 = 0.0
			amount float64
		)
		err := env.sqlf.
			From("incomes").
			Select("sum(amount) as got").To(&amount).
			GroupBy("user_id, from_user_id").
			OrderBy("got DESC").
			QueryAndClose(ctx, env.db, func(rows *sql.Rows) {
				nRows++
				total += amount
			})

		require.NoError(t, err, "Failed to execute a query. %s error: %v", env.driver, err)
		require.Equal(t, 4, nRows)
		require.Equal(t, 1550.0, total)
	})
}

var sqlSchemaCreate = []string{
	`CREATE TABLE users (
		id int IDENTITY PRIMARY KEY,
		name varchar(128) NOT NULL)`,
	`CREATE TABLE incomes (
		id int IDENTITY PRIMARY KEY,
		user_id int REFERENCES users(id),
		from_user_id int REFERENCES users(id),
		amount money)`,
}

var sqlFillDb = []string{
	`INSERT INTO users (id, name) VALUES (1, "User 1")`,
	`INSERT INTO users (id, name) VALUES (2, "User 2")`,
	`INSERT INTO users (id, name) VALUES (3, "User 3")`,

	`INSERT INTO incomes (user_id, from_user_id, amount) VALUES (1, 2, 100)`,
	`INSERT INTO incomes (user_id, from_user_id, amount) VALUES (1, 2, 200)`,
	`INSERT INTO incomes (user_id, from_user_id, amount) VALUES (1, 3, 350)`,
	`INSERT INTO incomes (user_id, from_user_id, amount) VALUES (2, 3, 400)`,
	`INSERT INTO incomes (user_id, from_user_id, amount) VALUES (3, 1, 500)`,
}

var sqlSchemaDrop = []string{
	`DROP TABLE incomes`,
	`DROP TABLE users`,
}
