package sqlf_test

import (
	"fmt"

	"github.com/leporo/sqlf"
)

func ExampleStmt_OrderBy() {
	q := sqlf.Select("id").From("table").OrderBy("id", "name DESC")
	fmt.Println(q.String())
	// Output: SELECT id FROM table ORDER BY id, name DESC
}

func ExampleStmt_Limit() {
	q := sqlf.Select("id").From("table").Limit(10)
	fmt.Println(q.String())
	// Output: SELECT id FROM table LIMIT ?
}

func ExampleStmt_Offset() {
	q := sqlf.Select("id").From("table").Limit(10).Offset(10)
	fmt.Println(q.String())
	// Output: SELECT id FROM table LIMIT ? OFFSET ?
}

func ExampleStmt_Paginate() {
	q := sqlf.Select("id").From("table").Paginate(5, 10)
	fmt.Println(q.String(), q.Args())
	q.Close()

	q = sqlf.Select("id").From("table").Paginate(1, 10)
	fmt.Println(q.String(), q.Args())
	q.Close()

	// Zero and negative values are replaced with 1
	q = sqlf.Select("id").From("table").Paginate(-1, -1)
	fmt.Println(q.String(), q.Args())
	q.Close()

	// Output:
	// SELECT id FROM table LIMIT ? OFFSET ? [10 40]
	// SELECT id FROM table LIMIT ? [10]
	// SELECT id FROM table LIMIT ? [1]
}

func ExampleStmt_Update() {
	q := sqlf.Update("table").Set("field1", "newvalue").Where("id = ?", 42)
	fmt.Println(q.String(), q.Args())
	q.Close()
	// Output:
	// UPDATE table SET field1=? WHERE id = ? [newvalue 42]
}

func ExampleStmt_SetExpr() {
	q := sqlf.Update("table").SetExpr("field1", "field2 + 1").Where("id = ?", 42)
	fmt.Println(q.String())
	fmt.Println(q.Args())
	q.Close()
	// Output:
	// UPDATE table SET field1=field2 + 1 WHERE id = ?
	// [42]
}

func ExampleStmt_InsertInto() {
	q := sqlf.InsertInto("table").
		Set("field1", "newvalue").
		SetExpr("field2", "field2 + 1")
	fmt.Println(q.String())
	fmt.Println(q.Args())
	q.Close()
	// Output:
	// INSERT INTO table ( field1, field2 ) VALUES ( ?, field2 + 1 )
	// [newvalue]
}

func ExampleStmt_DeleteFrom() {
	q := sqlf.DeleteFrom("table").Where("id = ?", 42)
	fmt.Println(q.String())
	fmt.Println(q.Args())
	q.Close()
	// Output:
	// DELETE FROM table WHERE id = ?
	// [42]
}

func ExampleStmt_GroupBy() {
	q := sqlf.From("incomes").
		Select("source, sum(amount) as s").
		Where("amount > ?", 42).
		GroupBy("source")
	fmt.Println(q.String())
	fmt.Println(q.Args())
	q.Close()
	// Output:
	// SELECT source, sum(amount) as s FROM incomes WHERE amount > ? GROUP BY source
	// [42]
}

func ExampleStmt_Having() {
	q := sqlf.From("incomes").
		Select("source, sum(amount) as s").
		Where("amount > ?", 42).
		GroupBy("source").
		Having("s > ?", 100)
	fmt.Println(q.String())
	fmt.Println(q.Args())
	q.Close()
	// Output:
	// SELECT source, sum(amount) as s FROM incomes WHERE amount > ? GROUP BY source HAVING s > ?
	// [42 100]
}

func ExampleStmt_Returning() {
	var newId int
	q := sqlf.InsertInto("table").
		Set("field1", "newvalue").
		Returning("id").To(&newId)
	fmt.Println(q.String(), q.Args())
	q.Close()
	// Output:
	// INSERT INTO table ( field1 ) VALUES ( ? ) RETURNING id [newvalue]
}

func ExamplePostgreSQL() {
	q := sqlf.PostgreSQL.From("table").Select("field").Where("id = ?", 42)
	fmt.Println(q.String())
	q.Close()
	// Output:
	// SELECT field FROM table WHERE id = $1
}

func ExampleSetDialect() {
	sqlf.SetDialect(sqlf.PostgreSQL)
	// ...
	sqlf.SetDialect(sqlf.NoDialect)
}

func ExampleStmt_With() {
	q := sqlf.From("orders").
		With("regional_sales", sqlf.From("orders").Select("region, SUM(amount) AS total_sales").GroupBy("region")).
		With("top_regions", sqlf.From("regional_sales").Select("region").Where("total_sales > (SELECT SUM(total_sales)/10 FROM regional_sales)")).
		Select("region").
		Select("product").
		Select("SUM(quantity) AS product_units").
		Select("SUM(amount) AS product_sales").
		Where("region IN (SELECT region FROM top_regions)").
		GroupBy("region, product")
	fmt.Println(q.String())
	q.Close()
	// Output:
	// WITH regional_sales AS (SELECT region, SUM(amount) AS total_sales FROM orders GROUP BY region), top_regions AS (SELECT region FROM regional_sales WHERE total_sales > (SELECT SUM(total_sales)/10 FROM regional_sales)) SELECT region, product, SUM(quantity) AS product_units, SUM(amount) AS product_sales FROM orders WHERE region IN (SELECT region FROM top_regions) GROUP BY region, product
}
