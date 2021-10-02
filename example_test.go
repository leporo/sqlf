package sqlf_test

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/leporo/sqlf"
)

type dummyDB int

func (db *dummyDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, nil
}

func (db *dummyDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (db *dummyDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

var db = new(dummyDB)
var ctx = context.Background()

func Example() {
	var (
		region       string
		product      string
		productUnits int
		productSales float64
	)

	sqlf.SetDialect(sqlf.PostgreSQL)

	err := sqlf.From("orders").
		With("regional_sales",
			sqlf.From("orders").
				Select("region, SUM(amount) AS total_sales").
				GroupBy("region")).
		With("top_regions",
			sqlf.From("regional_sales").
				Select("region").
				Where("total_sales > (SELECT SUM(total_sales)/10 FROM regional_sales)")).
		// Map query fields to variables
		Select("region").To(&region).
		Select("product").To(&product).
		Select("SUM(quantity)").To(&productUnits).
		Select("SUM(amount) AS product_sales").To(&productSales).
		//
		Where("region IN (SELECT region FROM top_regions)").
		GroupBy("region, product").
		OrderBy("product_sales DESC").
		// Execute the query
		QueryAndClose(ctx, db, func(row *sql.Rows) {
			// Callback function is called for every returned row.
			// Row values are scanned automatically to bound variables.
			fmt.Printf("%s\t%s\t%d\t$%.2f\n", region, product, productUnits, productSales)
		})
	if err != nil {
		panic(err)
	}
}

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

func ExampleStmt_With() {
	q := sqlf.From("orders").
		With("regional_sales",
			sqlf.From("orders").
				Select("region, SUM(amount) AS total_sales").
				GroupBy("region")).
		With("top_regions",
			sqlf.From("regional_sales").
				Select("region").
				Where("total_sales > (SELECT SUM(total_sales)/10 FROM regional_sales)")).
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

func ExampleStmt_From() {
	q := sqlf.Select("*").
		From("").
		SubQuery(
			"(", ") counted_news",
			sqlf.From("news").
				Select("id, section, header, score").
				Select("row_number() OVER (PARTITION BY section ORDER BY score DESC) AS rating_in_section").
				OrderBy("section, rating_in_section")).
		Where("rating_in_section <= 5")
	fmt.Println(q.String())
	q.Close()
	// Output:
	//SELECT * FROM (SELECT id, section, header, score, row_number() OVER (PARTITION BY section ORDER BY score DESC) AS rating_in_section FROM news ORDER BY section, rating_in_section) counted_news WHERE rating_in_section <= 5
}

func ExampleStmt_SubQuery() {
	q := sqlf.From("orders o").
		Select("date, region").
		SubQuery("(", ") AS prev_order_date",
			sqlf.From("orders po").
				Select("date").
				Where("region = o.region").
				Where("id < o.id").
				OrderBy("id DESC").
				Clause("LIMIT 1")).
		Where("date > CURRENT_DATE - interval '1 day'").
		OrderBy("id DESC")
	fmt.Println(q.String())
	q.Close()

	// Output:
	// SELECT date, region, (SELECT date FROM orders po WHERE region = o.region AND id < o.id ORDER BY id DESC LIMIT 1) AS prev_order_date FROM orders o WHERE date > CURRENT_DATE - interval '1 day' ORDER BY id DESC
}

func ExampleStmt_Clause() {
	q := sqlf.From("empsalary").
		Select("sum(salary) OVER w").
		Clause("WINDOW w AS (PARTITION BY depname ORDER BY salary DESC)")
	fmt.Println(q.String())
	q.Close()

	// Output:
	// SELECT sum(salary) OVER w FROM empsalary WINDOW w AS (PARTITION BY depname ORDER BY salary DESC)
}

func ExampleStmt_QueryRowAndClose() {
	type Offer struct {
		id        int64
		productId int64
		price     float64
		isDeleted bool
	}

	var o Offer

	err := sqlf.From("offers").
		Select("id").To(&o.id).
		Select("product_id").To(&o.productId).
		Select("price").To(&o.price).
		Select("is_deleted").To(&o.isDeleted).
		Where("id = ?", 42).
		QueryRowAndClose(ctx, db)
	if err != nil {
		panic(err)
	}
}

func ExampleStmt_Bind() {
	type Offer struct {
		Id        int64   `db:"id"`
		ProductId int64   `db:"product_id"`
		Price     float64 `db:"price"`
		IsDeleted bool    `db:"is_deleted"`
	}

	var o Offer

	err := sqlf.From("offers").
		Bind(&o).
		Where("id = ?", 42).
		QueryRowAndClose(ctx, db)
	if err != nil {
		panic(err)
	}
}

func ExampleStmt_In() {
	q := sqlf.From("tasks").
		Select("id, status").
		Where("status").In("new", "pending", "wip")
	fmt.Println(q.String())
	fmt.Println(q.Args())
	q.Close()

	// Output:
	// SELECT id, status FROM tasks WHERE status IN (?,?,?)
	// [new pending wip]
}

func ExampleStmt_Union() {
	q := sqlf.From("tasks").
		Select("id, status").
		Where("status = ?", "new").
		Union(true, sqlf.From("tasks").
			Select("id, status").
			Where("status = ?", "pending")).
		Union(true, sqlf.From("tasks").
			Select("id, status").
			Where("status = ?", "wip")).
		OrderBy("id")
	fmt.Println(q.String())
	fmt.Println(q.Args())
	q.Close()

	// Output:
	// SELECT id, status FROM tasks WHERE status = ? UNION ALL SELECT id, status FROM tasks WHERE status = ? UNION ALL SELECT id, status FROM tasks WHERE status = ? ORDER BY id
	// [new pending wip]
}
