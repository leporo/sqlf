package sqlf_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/leporo/sqlf"
	"github.com/stretchr/testify/require"
)

func TestNewBuilder(t *testing.T) {
	sqlf.SetDialect(sqlf.NoDialect)
	q := sqlf.New("SELECT *").From("table")
	defer q.Close()
	sql := q.String()
	args := q.Args()
	require.Equal(t, "SELECT * FROM table", sql)
	require.Empty(t, args)
}

func TestBasicSelect(t *testing.T) {
	q := sqlf.From("table").Select("id").Where("id > ?", 42).Where("id < ?", 1000)
	defer q.Close()
	sql, args := q.String(), q.Args()
	require.Equal(t, "SELECT id FROM table WHERE id > ? AND id < ?", sql)
	require.Equal(t, []interface{}{42, 1000}, args)
}

func TestMixedOrder(t *testing.T) {
	q := sqlf.Select("id").Where("id > ?", 42).From("table").Where("id < ?", 1000)
	defer q.Close()
	sql, args := q.String(), q.Args()
	require.Equal(t, "SELECT id FROM table WHERE id > ? AND id < ?", sql)
	require.Equal(t, []interface{}{42, 1000}, args)
}

func TestClause(t *testing.T) {
	q := sqlf.Select("id").From("table").Where("id > ?", 42).Clause("FETCH NEXT").Clause("FOR UPDATE")
	defer q.Close()
	sql, args := q.String(), q.Args()
	require.Equal(t, "SELECT id FROM table WHERE id > ? FETCH NEXT FOR UPDATE", sql)
	require.Equal(t, []interface{}{42}, args)
}

func TestExpr(t *testing.T) {
	q := sqlf.From("table").
		Select("id").
		Expr("(select 1 from related where table_id = table.id limit 1) AS has_related").
		Where("id > ?", 42)
	require.Equal(t, "SELECT id, (select 1 from related where table_id = table.id limit 1) AS has_related FROM table WHERE id > ?", q.String())
	require.Equal(t, []interface{}{42}, q.Args())
	q.Close()
}

func TestManyFields(t *testing.T) {
	q := sqlf.Select("id").From("table").Where("id = ?", 42)
	defer q.Close()
	for i := 1; i <= 3; i++ {
		q.Select(fmt.Sprintf("(id + ?) as id_%d", i), i*10)
	}
	for _, field := range []string{"uno", "dos", "tres"} {
		q.Select(field)
	}
	sql, args := q.String(), q.Args()
	require.Equal(t, "SELECT id, (id + ?) as id_1, (id + ?) as id_2, (id + ?) as id_3, uno, dos, tres FROM table WHERE id = ?", sql)
	require.Equal(t, []interface{}{10, 20, 30, 42}, args)
}

func TestEvenMoreFields(t *testing.T) {
	q := sqlf.Select("id").From("table")
	defer q.Close()
	for n := 1; n <= 50; n++ {
		q.Select(fmt.Sprintf("field_%d", n))
	}
	sql, args := q.String(), q.Args()
	require.Equal(t, 0, len(args))
	for n := 1; n <= 50; n++ {
		field := fmt.Sprintf(", field_%d", n)
		require.Contains(t, sql, field)
	}
}

func TestPgPlaceholders(t *testing.T) {
	q := sqlf.PostgreSQL.From("series").
		Select("id").
		Where("time > ?", time.Now().Add(time.Hour*-24*14)).
		Where("(time < ?)", time.Now().Add(time.Hour*-24*7))
	defer q.Close()
	sql, _ := q.String(), q.Args()
	require.Equal(t, "SELECT id FROM series WHERE time > $1 AND (time < $2)", sql)
}

func TestPgPlaceholderEscape(t *testing.T) {
	q := sqlf.PostgreSQL.From("series").
		Select("id").
		Where("time \\?> ? + 1", time.Now().Add(time.Hour*-24*14)).
		Where("time < ?", time.Now().Add(time.Hour*-24*7))
	defer q.Close()
	sql, _ := q.String(), q.Args()
	require.Equal(t, "SELECT id FROM series WHERE time ?> $1 + 1 AND time < $2", sql)
}

func TestTo(t *testing.T) {
	var (
		field1 int
		field2 string
	)
	q := sqlf.From("table").
		Select("field1").To(&field1).
		Select("field2").To(&field2)
	defer q.Close()
	dest := q.Dest()

	require.Equal(t, []interface{}{&field1, &field2}, dest)
}

func TestManyClauses(t *testing.T) {
	q := sqlf.From("table").
		Select("field").
		Where("id > ?", 2).
		Clause("UNO").
		Clause("DOS").
		Clause("TRES").
		Clause("QUATRO").
		Offset(10).
		Limit(5).
		Clause("NO LOCK")
	defer q.Close()
	sql, args := q.String(), q.Args()

	require.Equal(t, "SELECT field FROM table WHERE id > ? UNO DOS TRES QUATRO LIMIT ? OFFSET ? NO LOCK", sql)
	require.Equal(t, []interface{}{2, 5, 10}, args)
}

func TestWith(t *testing.T) {
	var row struct {
		ID       int64 `db:"id"`
		Quantity int64 `db:"quantity"`
	}
	q := sqlf.With("t",
		sqlf.From("orders").
			Select("id, quantity").
			Where("ts < ?", time.Now())).
		From("t").
		Bind(&row)
	defer q.Close()

	require.Equal(t, "WITH t AS (SELECT id, quantity FROM orders WHERE ts < ?) SELECT id, quantity FROM t", q.String())
}

func TestWithRecursive(t *testing.T) {
	q := sqlf.From("orders").
		With("RECURSIVE regional_sales", sqlf.From("orders").Select("region, SUM(amount) AS total_sales").GroupBy("region")).
		With("top_regions", sqlf.From("regional_sales").Select("region").OrderBy("total_sales DESC").Limit(5)).
		Select("region").
		Select("product").
		Select("SUM(quantity) AS product_units").
		Select("SUM(amount) AS product_sales").
		Where("region IN (SELECT region FROM top_regions)").
		GroupBy("region, product")
	defer q.Close()

	require.Equal(t, "WITH RECURSIVE regional_sales AS (SELECT region, SUM(amount) AS total_sales FROM orders GROUP BY region), top_regions AS (SELECT region FROM regional_sales ORDER BY total_sales DESC LIMIT ?) SELECT region, product, SUM(quantity) AS product_units, SUM(amount) AS product_sales FROM orders WHERE region IN (SELECT region FROM top_regions) GROUP BY region, product", q.String())
}

func TestSubQueryDialect(t *testing.T) {
	q := sqlf.PostgreSQL.From("users u").
		Select("email").
		Where("registered > ?", "2019-01-01").
		SubQuery("EXISTS (", ")",
			sqlf.PostgreSQL.From("orders").
				Select("id").
				Where("user_id = u.id").
				Where("amount > ?", 100))
	defer q.Close()

	// Parameter placeholder numbering should match the arguments
	require.Equal(t, "SELECT email FROM users u WHERE registered > $1 AND EXISTS (SELECT id FROM orders WHERE user_id = u.id AND amount > $2)", q.String())
	require.Equal(t, []interface{}{"2019-01-01", 100}, q.Args())
}

func TestClone(t *testing.T) {
	var (
		value  string
		value2 string
	)
	q := sqlf.From("table").Select("field").To(&value).Where("id = ?", 42)
	defer q.Close()

	require.Equal(t, "SELECT field FROM table WHERE id = ?", q.String())

	q2 := q.Clone()
	defer q2.Close()

	require.Equal(t, q.Args(), q2.Args())
	require.Equal(t, q.Dest(), q2.Dest())
	require.Equal(t, q.String(), q2.String())

	q2.Where("time < ?", time.Now())

	require.Equal(t, q.Dest(), q2.Dest())
	require.NotEqual(t, q.Args(), q2.Args())
	require.NotEqual(t, q.String(), q2.String())

	q2.Select("field2").To(&value2)
	require.NotEqual(t, q.Dest(), q2.Dest())
	require.NotEqual(t, q.Args(), q2.Args())
	require.NotEqual(t, q.String(), q2.String())

	// Add more clauses to original statement to re-allocate chunks array
	q.With("top_users", sqlf.From("users").OrderBy("rating DESCT").Limit(10)).
		GroupBy("id").
		Having("field > ?", 10).
		Paginate(1, 20).
		Clause("FETCH ROWS ONLY").
		Clause("FOR UPDATE")

	q3 := q.Clone()
	require.Equal(t, q.Args(), q3.Args())
	require.Equal(t, q.Dest(), q3.Dest())
	require.Equal(t, q.String(), q3.String())

	require.NotEqual(t, q.Dest(), q2.Dest())
	require.NotEqual(t, q.Args(), q2.Args())
	require.NotEqual(t, q.String(), q2.String())
}

func TestJoin(t *testing.T) {
	q := sqlf.From("orders o").Select("id").Join("users u", "u.id = o.user_id")
	defer q.Close()
	require.Equal(t, "SELECT id FROM orders o JOIN users u ON (u.id = o.user_id)", q.String())
}

func TestLeftJoin(t *testing.T) {
	q := sqlf.From("orders o").Select("id").LeftJoin("users u", "u.id = o.user_id")
	defer q.Close()
	require.Equal(t, "SELECT id FROM orders o LEFT JOIN users u ON (u.id = o.user_id)", q.String())
}

func TestRightJoin(t *testing.T) {
	q := sqlf.From("orders o").Select("id").RightJoin("users u", "u.id = o.user_id")
	defer q.Close()
	require.Equal(t, "SELECT id FROM orders o RIGHT JOIN users u ON (u.id = o.user_id)", q.String())
}

func TestFullJoin(t *testing.T) {
	q := sqlf.From("orders o").Select("id").FullJoin("users u", "u.id = o.user_id")
	defer q.Close()
	require.Equal(t, "SELECT id FROM orders o FULL JOIN users u ON (u.id = o.user_id)", q.String())
}

func TestUnion(t *testing.T) {
	q := sqlf.From("tasks").
		Select("id, status").
		Where("status = ?", "new").
		Union(false, sqlf.PostgreSQL.From("tasks").
			Select("id, status").
			Where("status = ?", "wip"))
	defer q.Close()
	require.Equal(t, "SELECT id, status FROM tasks WHERE status = ? UNION SELECT id, status FROM tasks WHERE status = ?", q.String())
}

func TestLimit(t *testing.T) {
	q := sqlf.From("items").
		Select("id").
		Where("id > ?", 42).
		Limit(10).
		Limit(11).
		Limit(20)
	defer q.Close()
	require.Equal(t, "SELECT id FROM items WHERE id > ? LIMIT ?", q.String())
	require.Equal(t, []interface{}{42, 20}, q.Args())
}

func TestBindStruct(t *testing.T) {
	type Parent struct {
		ID      int64     `db:"id"`
		Date    time.Time `db:"date"`
		Skipped string
	}
	var u struct {
		Parent
		ChildTime time.Time `db:"child_time"`
		Name      string    `db:"name"`
		Extra     int64
	}
	q := sqlf.From("users").
		Bind(&u).
		Where("id = ?", 2)
	defer q.Close()
	require.Equal(t, "SELECT id, date, child_time, name FROM users WHERE id = ?", q.String())
	require.Equal(t, []interface{}{2}, q.Args())
	require.EqualValues(t, []interface{}{&u.ID, &u.Date, &u.ChildTime, &u.Name}, q.Dest())
}
