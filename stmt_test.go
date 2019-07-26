package sqlf_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/leporo/sqlf"
	"github.com/stretchr/testify/assert"
)

func TestNewBuilder(t *testing.T) {
	sqlf.SetDialect(sqlf.NoDialect)
	q := sqlf.New("SELECT *").From("table")
	defer q.Close()
	sql := q.String()
	args := q.Args()
	assert.Equal(t, "SELECT * FROM table", sql)
	assert.Empty(t, args)
}

func TestBasicSelect(t *testing.T) {
	q := sqlf.From("table").Select("id").Where("id > ?", 42).Where("id < ?", 1000)
	defer q.Close()
	sql, args := q.String(), q.Args()
	assert.Equal(t, "SELECT id FROM table WHERE id > ? AND id < ?", sql)
	assert.Equal(t, []interface{}{42, 1000}, args)
}

func TestMixedOrder(t *testing.T) {
	q := sqlf.Select("id").Where("id > ?", 42).From("table").Where("id < ?", 1000)
	defer q.Close()
	sql, args := q.SQL(), q.Args()
	assert.Equal(t, "SELECT id FROM table WHERE id > ? AND id < ?", sql)
	assert.Equal(t, []interface{}{42, 1000}, args)
}

func TestClause(t *testing.T) {
	q := sqlf.Select("id").From("table").Where("id > ?", 42).Clause("FETCH NEXT").Clause("FOR UPDATE")
	defer q.Close()
	sql, args := q.SQL(), q.Args()
	assert.Equal(t, "SELECT id FROM table WHERE id > ? FETCH NEXT FOR UPDATE", sql)
	assert.Equal(t, []interface{}{42}, args)
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
	sql, args := q.SQL(), q.Args()
	assert.Equal(t, "SELECT id, (id + ?) as id_1, (id + ?) as id_2, (id + ?) as id_3, uno, dos, tres FROM table WHERE id = ?", sql)
	assert.Equal(t, []interface{}{10, 20, 30, 42}, args)
}

func TestEvenMoreFields(t *testing.T) {
	q := sqlf.Select("id").From("table")
	defer q.Close()
	for n := 1; n <= 50; n++ {
		q.Select(fmt.Sprintf("field_%d", n))
	}
	sql, args := q.SQL(), q.Args()
	assert.Equal(t, 0, len(args))
	for n := 1; n <= 50; n++ {
		field := fmt.Sprintf(", field_%d", n)
		assert.Contains(t, sql, field)
	}
}

func TestPgPlaceholders(t *testing.T) {
	q := sqlf.PostgreSQL.From("series").
		Select("id").
		Where("time > ?", time.Now().Add(time.Hour*-24*14)).
		Where("(time < ?)", time.Now().Add(time.Hour*-24*7))
	defer q.Close()
	sql, _ := q.SQL(), q.Args()
	assert.Equal(t, "SELECT id FROM series WHERE time > $1 AND (time < $2)", sql)
}

func TestPgPlaceholderEscape(t *testing.T) {
	q := sqlf.PostgreSQL.From("series").
		Select("id").
		Where("time \\?> ? + 1", time.Now().Add(time.Hour*-24*14)).
		Where("time < ?", time.Now().Add(time.Hour*-24*7))
	defer q.Close()
	sql, _ := q.SQL(), q.Args()
	assert.Equal(t, "SELECT id FROM series WHERE time ?> $1 + 1 AND time < $2", sql)
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

	assert.Equal(t, []interface{}{&field1, &field2}, dest)
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
	sql, args := q.SQL(), q.Args()

	assert.Equal(t, "SELECT field FROM table WHERE id > ? UNO DOS TRES QUATRO LIMIT ? OFFSET ? NO LOCK", sql)
	assert.Equal(t, []interface{}{2, 5, 10}, args)
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

	assert.Equal(t, "WITH RECURSIVE regional_sales AS (SELECT region, SUM(amount) AS total_sales FROM orders GROUP BY region), top_regions AS (SELECT region FROM regional_sales ORDER BY total_sales DESC LIMIT ?) SELECT region, product, SUM(quantity) AS product_units, SUM(amount) AS product_sales FROM orders WHERE region IN (SELECT region FROM top_regions) GROUP BY region, product", q.String())
}
