# sqlf

[![GoDoc Reference](https://godoc.org/github.com/leporo/sqlf?status.svg)](http://godoc.org/github.com/leporo/sqlf)
[![Build Status](https://travis-ci.org/leporo/sqlf.svg?branch=master)](https://travis-ci.org/leporo/sqlf)
[![Go Report Card](https://goreportcard.com/badge/github.com/leporo/sqlf)](https://goreportcard.com/report/github.com/leporo/sqlf)


A fast and flexible SQL query builder for Go.

`sqlf` statement builder provides a way to:
- Combine SQL statements from fragments of raw SQL and arguments that match
  those fragments,
- Map columns to variables to be referenced by Scan,
- Convert ? placeholders into numbered ones for PostgreSQL ($1, $2, etc).

`sqlf.Stmt` has methods to execute a query using any database/sql compatible driver.

It can be used along with `sqlx` or other library.

## Is It Fast?

It is. See benchmarks: https://github.com/leporo/golang-sql-builder-benchmark

In order to maximize performance and minimize memory footprint, `sqlf` reuses memory allocated for query building. The heavier load is, the faster `sqlf` works.

## Has It to Be Fast?

Any SQL builder or ORM is there for developer's sake. It makes it easier to develop and maintain applications but takes its toll in processing time. It's a trade-off and there are no ultimate answers. Regardless of query builder used, a database query execution takes way more time than query building and processing.

Check it. Test it. It's up to you to decide.

## Usage

```go
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
```

Build complex statements:

```go
var (
    region       string
    product      string
    productUnits int
    productSales float64
)

err := sqlf.From("orders").
    With("regional_sales",
        sqlf.From("orders").
            Select("region, SUM(amount) AS total_sales").
            GroupBy("region")).
    With("top_regions",
        sqlf.From("regional_sales").
            Select("region").
            Where("total_sales > (SELECT SUM(total_sales)/10 FROM regional_sales)")).
    Select("region").To(&region).
    Select("product").To(&product).
    Select("SUM(quantity)").To(&productUnits).
    Select("SUM(amount) AS product_sales").To(&productSales).
    Where("region IN (SELECT region FROM top_regions)").
    GroupBy("region, product").
    OrderBy("product_sales DESC").
    QueryAndClose(ctx, db, func(row *sql.Rows){
        fmt.Printf("%s\t%s\t%d\t$%.2f\n", region, product, productUnits, productSales)
    })
if err != nil {
    panic(err)
}
```

Some SQL fragments, like a list of fields to be selected or filtering condition may appear over and over. It can be annoying to repeat them or combine an SQL statement from chunks. Use `sqlf.Stmt` to construct a basic query and extend it for a case:

```go
func (o *Offer) Select() *sqlf.Stmt {
    return sqlf.From("products").
        Select("id").To(&p.id).
        Select("product_id").To(&p.productId).
        Select("price").To(&p.price).
        Select("is_deleted").To(&p.isDeleted).
        // Ignore deleted offers
        Where("is_deleted = false")
}

func (o Offer) Print() {
    fmt.Printf("%d\t%s\t$%.2f\n", o.id, o.name, o.price)
}

var o Offer

// Fetch offer data
err := o.Select().
    Where("id = ?", offerId).
    QueryRowAndClose(ctx, db)
if err != nil {
    panic(err)
}
o.Print()
// ...

// Select and print 5 most recently placed
// offers for a given product
err = o.Select().
    Where("product_id = ?", productId).
    OrderBy("id DESC").
    Limit(5).
    QueryAndClose(ctx, db, func(row *sql.Rows){
        o.Print()
    })
if err != nil {
    panic(err)
}
// ...

```

## SQL Statement Construction and Execution

### Thou Shalt Learn SQL

`sqlf` is a query builder. It's there to help you construct SQL statements and process the returned results. It won't jump in and set an extra level of abstraction between application and database server. You have to use a SQL dialect of your database server.

Good news is you don't need to look for a method to use a specific database server feature, like array fields or full text search. Just use SQL.

`sqlf` allows you to split a complex statement into fragments, manage those fragments and combine it all together before execution. There are methods for most common SQL statement clauses but don't expect it to cover operators or built-in functions like `current_timestamp` or `array_agg()`.

### SELECT

#### Value Binding

Bind columns to values using `To` method:

```go
var (
    minAmountRequested = true
    maxAmount float64
    minAmount float64
)

q := sqlf.From("offers").
    Select("MAX(amount)").To(&maxAmount).
    Where("is_deleted = false")

if minAmountRequested {
    q.Select("MIN(amount)").To(&minAmount)
}

err := q.QueryRowAndClose(ctx, db)
if err != nil {
    panic(err)
}
if minAmountRequested {
    fmt.Printf("Cheapest offer: $%.2f\n", minAmount)
}
fmt.Printf("Most expensive offer: $%.2f\n", minAmount)
```

#### Joins

There are no helper methods to construct a JOIN clause.

Consider using the "old style" syntax for INNER JOINs:

```go
var (
    offerId     int64
    productName string
    price       float64
}

err := sqlf.From("offers o").
    Select("o.id").To(&offerId).
    Select("price").To(&price).
    Where("is_deleted = false").
    // Join
    From("products p").
    Where("p.id = o.product_id").
    // Bind a column from joined table to variable
    Select("p.name").To(&productName).
    // Print top 10 offers
    OrderBy("price DEST").
    Limit(10).
    QueryAndClose(ctx, db, func(row *sql.Rows){
        fmt.Printf("%d\t%s\t$%.2f\n", offerId, productName, price)
    })
if err != nil {
    panic(err)
}
```

Use plain SQL for more fancy cases:

```go
var (
    num   int64
    name  string
    value string
)
err := sqlf.From("t1 FULL JOIN t2 ON t1.num = t2.num AND t2.value IN (?, ?)", "xxx", "yyy").
    Select("t1.num").To(&num).
    Select("t1.name").To(&name).
    Select("t2.value").To(&value).
    QueryAndClose(ctx, db, func(row *sql.Rows){
        fmt.Printf("%d\t%s\ts\n", num, name, value)
    })
if err != nil {
    panic(err)
}
```

#### Subqueries

Use `SubQuery` method to add a sub query to a statement:

```go
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
	// SELECT
    //   date,
    //   region,
    //   (SELECT date FROM orders po WHERE region = o.region AND id < o.id ORDER BY id DESC LIMIT 1) AS prev_order_date
    // FROM orders o
    // WHERE date > CURRENT_DATE - interval '1 day'
    // ORDER BY id DESC
```

Not that if a subquery uses no arguments like it's more effective to add it as SQL fragment:

```go
	q := sqlf.From("orders o").
		Select("date, region").
		Where("date > CURRENT_DATE - interval '1 day'").
        Where("exists (SELECT 1 FROM orders po WHERE region = o.region AND id < o.id ORDER BY id DESC LIMIT 1)").
		OrderBy("id DESC")
```

### INSERT

A simple INSERT statement execution using the `database/sql` standard library may look like this:

```go
_, err := db.ExecContext(ctx, "INSERT INTO users (email, address) VALUES ($1, $2)", "new@email.com", "320 Some Avenue, Somewhereville, GA, US")
```

There are just 2 fields of a new database record to be populated, and yet it takes some time to figure out what those fields are and what values are to be assigned to them.

In real-world cases there are tens of fields. On any update both the list of field names and the list of values, passed to `ExecContext` method, have to to be reviewed and updated. It's a common thing to have values misplaced or shuffle the list of fields.

`sqlf` provides a `Set` method to be used both for UPDATE and INSERT statements. It helps to make sure values do match fields:

```go
var userId int64

err := sqlf.InsertInto("users").
    Set("email", "new@email.com").
    Set("address", "320 Some Avenue, Somewhereville, GA, US").
    Returning("id").To(&userId).
    Clause("ON CONFLICT (email) DO UPDATE SET address = users.address").
    ExecAndClose(ctx, db)
```

### UPDATE

```go
err := sqlf.Update("users").
    Set("email", "new@email.com").
    ExecAndClose(ctx, db)
```

### DELETE

```go
err := sqlf.DeleteFrom("products").
    Where("id = ?", 42)
    ExecAndClose(ctx, db)
```

`sqlf` doesn't implement any method to populate a structure with matching column values. There is a lot of battle-tested libraries like `sqlx` and `dbr` that do it.
