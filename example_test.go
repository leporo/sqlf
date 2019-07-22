package sqlf_test

import (
	"fmt"

	"github.com/leporo/sqlf"
)

func ExampleStmt_OrderBy() {
	q := sqlf.Select("id").From("table").OrderBy("id", "name DESC")
	sql, _ := q.Build()
	fmt.Println(sql)
	// Output: SELECT id FROM table ORDER BY id, name DESC
}

func ExampleStmt_Limit() {
	q := sqlf.Select("id").From("table").Limit(10)
	sql, _ := q.Build()
	fmt.Println(sql)
	// Output: SELECT id FROM table LIMIT ?
}

func ExampleStmt_Offset() {
	q := sqlf.Select("id").From("table").Limit(10).Offset(10)
	sql, _ := q.Build()
	fmt.Println(sql)
	// Output: SELECT id FROM table LIMIT ? OFFSET ?
}

func ExampleStmt_Paginate() {
	sql, args := sqlf.Select("id").From("table").Paginate(5, 10).Build()
	fmt.Println(sql, args)
	sql, args = sqlf.Select("id").From("table").Paginate(1, 10).Build()
	fmt.Println(sql, args)
	// Zero and negative values are replaced with 1
	sql, args = sqlf.Select("id").From("table").Paginate(-1, -1).Build()
	fmt.Println(sql, args)
	// Output:
	// SELECT id FROM table LIMIT ? OFFSET ? [10 40]
	// SELECT id FROM table LIMIT ? [10]
	// SELECT id FROM table LIMIT ? [1]
}

func ExampleStmt_Update() {
	sql, args := sqlf.Update("table").Set("field1", "newvalue").Where("id = ?", 42).Build()
	fmt.Println(sql)
	fmt.Println(args)
	// Output:
	// UPDATE table SET field1=? WHERE id = ?
	// [newvalue 42]
}

func ExampleStmt_SetExpr() {
	sql, args := sqlf.Update("table").SetExpr("field1", "field2 + 1").Where("id = ?", 42).Build()
	fmt.Println(sql)
	fmt.Println(args)
	// Output:
	// UPDATE table SET field1=field2 + 1 WHERE id = ?
	// [42]
}

func ExampleStmt_InsertInto() {
	sql, args := sqlf.InsertInto("table").
		Set("field1", "newvalue").
		SetExpr("field2", "field2 + 1").
		Build()
	fmt.Println(sql)
	fmt.Println(args)
	// Output:
	// INSERT INTO table ( field1, field2 ) VALUES ( ?, field2 + 1 )
	// [newvalue]
}

func ExampleStmt_DeleteFrom() {
	sql, args := sqlf.DeleteFrom("table").Where("id = ?", 42).Build()
	fmt.Println(sql)
	fmt.Println(args)
	// Output:
	// DELETE FROM table WHERE id = ?
	// [42]
}

func ExampleStmt_GroupBy() {
	sql, args := sqlf.From("incomes").
		Select("source, sum(amount) as s").
		Where("amount > ?", 42).
		GroupBy("source").
		Build()
	fmt.Println(sql)
	fmt.Println(args)
	// Output:
	// SELECT source, sum(amount) as s FROM incomes WHERE amount > ? GROUP BY source
	// [42]
}

func ExampleStmt_Having() {
	sql, args := sqlf.From("incomes").
		Select("source, sum(amount) as s").
		Where("amount > ?", 42).
		GroupBy("source").
		Having("s > ?", 100).
		Build()
	fmt.Println(sql)
	fmt.Println(args)
	// Output:
	// SELECT source, sum(amount) as s FROM incomes WHERE amount > ? GROUP BY source HAVING s > ?
	// [42 100]
}

func ExampleStmt_Returning() {
	var newId int
	sql, args := sqlf.InsertInto("table").
		Set("field1", "newvalue").
		Returning("id").To(&newId).
		Build()
	fmt.Println(sql)
	fmt.Println(args)
	// Output:
	// INSERT INTO table ( field1 ) VALUES ( ? ) RETURNING id
	// [newvalue]
}

func ExampleNewBuilder() {
	qb := sqlf.NewBuilder(sqlf.PostgreSQL())
	q := qb.From("table").Select("field").Where("id = ?", 42)
	sql, _ := q.Build()
	q.Close()
	fmt.Println(sql)
	// Output:
	// SELECT field FROM table WHERE id = $1
}

func ExampleSetDialect() {
	sqlf.SetDialect(sqlf.PostgreSQL())
	sqlf.SetDialect(sqlf.NoDialect())
}
