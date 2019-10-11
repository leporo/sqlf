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

## Is It Fast?

It is. See benchmarks: https://github.com/leporo/golang-sql-builder-benchmark

In order to maximize performance and minimize memory footprint, `sqlf` reuses memory allocated for query building. The heavier load is, the faster `sqlf` works.

## Usage

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

Bind structures to query results:

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

There are helper methods to construct a JOIN clause: `Join`, `LeftJoin`, `RightJoin` and `FullJoin`.

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
    LeftJoin("products p", "p.id = o.product_id").
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
err := sqlf.From("t1 CROSS JOIN t2 ON t1.num = t2.num AND t2.value IN (?, ?)", "xxx", "yyy").
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
```

Note that if a subquery uses no arguments, it's more effective to add it as SQL fragment:

```go
	q := sqlf.From("orders o").
		Select("date, region").
		Where("date > CURRENT_DATE - interval '1 day'").
        Where("exists (SELECT 1 FROM orders po WHERE region = o.region AND id < o.id ORDER BY id DESC LIMIT 1)").
        OrderBy("id DESC")
    // ...
    q.Close()
```

To select from sub-query pass an empty string to From and immediately call a SubQuery method.

The query constructed by the following example returns top 5 news in each section:

```go
	q := sqlf.Select("").
		From("").
		SubQuery(
			"(", ") counted_news",
			sqlf.From("news").
				Select("id, section, header, score").
				Select("row_number() OVER (PARTITION BY section ORDER BY score DESC) AS rating_in_section").
				OrderBy("section, rating_in_section")).
		Where("rating_in_section <= 5")
    // ...
    q.Close()
```

#### Unions

Use `Union` method to combine results of two queries:

```go
	q := sqlf.From("tasks").
		Select("id, status").
		Where("status = ?", "new").
		Union(true, sqlf.PostgreSQL.From("tasks").
			Select("id, status").
            Where("status = ?", "wip"))
    // ...
	q.Close()
```

### INSERT

`sqlf` provides a `Set` method to be used both for UPDATE and INSERT statements:

```go
var userId int64

err := sqlf.InsertInto("users").
    Set("email", "new@email.com").
    Set("address", "320 Some Avenue, Somewhereville, GA, US").
    Returning("id").To(&userId).
    Clause("ON CONFLICT (email) DO UPDATE SET address = users.address").
    ExecAndClose(ctx, db)
```

The same statement execution using the `database/sql` standard library looks like this:

```go
var userId int64

// database/sql
err := db.ExecContext(ctx, "INSERT INTO users (email, address) VALUES ($1, $2) RETURNING id ON CONFLICT (email) DO UPDATE SET address = users.address", "new@email.com", "320 Some Avenue, Somewhereville, GA, US").Scan(&userId)
```

There are just 2 fields of a new database record to be populated, and yet it takes some time to figure out what columns are being updated and what values are to be assigned to them.

In real-world cases there are tens of fields. On any update both the list of field names and the list of values, passed to `ExecContext` method, have to to be reviewed and updated. It's a common thing to have values misplaced.

The use of `Set` method to maintain a field-value map is a way to solve this issue.

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
