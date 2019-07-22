package sqlf_test

import (
	"fmt"
	"testing"

	"github.com/leporo/sqlf"
)

var builderNo = sqlf.NewBuilder(sqlf.NoDialect())
var builderPg = sqlf.NewBuilder(sqlf.PostgreSQL())

func BenchmarkSelectDontClose(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := sqlf.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)
		q.Build()
	}
}

func BenchmarkSelect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := sqlf.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)
		q.Build()
		q.Close()
	}
}

func BenchmarkSelectPg(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := builderPg.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)
		q.Build()
		q.Close()
	}
}

func BenchmarkManyFields(b *testing.B) {
	fields := make([]string, 0, 100)

	for n := 1; n <= cap(fields); n++ {
		fields = append(fields, fmt.Sprintf("field_%d", n))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		q := sqlf.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)
		for _, field := range fields {
			q.Select(field)
		}
		q.Build()
		q.Close()
	}
}

func BenchmarkManyFieldsPg(b *testing.B) {
	fields := make([]string, 0, 100)

	for n := 1; n <= cap(fields); n++ {
		fields = append(fields, fmt.Sprintf("field_%d", n))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		q := builderPg.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)
		for _, field := range fields {
			q.Select(field)
		}
		q.Build()
		q.Close()
	}
}

func BenchmarkMixedOrder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := sqlf.Select("id").Where("id > ?", 42).From("table").Where("id < ?", 1000)
		q.Build()
		q.Close()
	}
}

func BenchmarkBuildPg(b *testing.B) {
	q := builderPg.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)

	for i := 0; i < b.N; i++ {
		q.Invalidate()
		q.Build()
	}
}

func BenchmarkBuild(b *testing.B) {
	q := sqlf.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)

	for i := 0; i < b.N; i++ {
		q.Invalidate()
		q.Build()
	}
}

func BenchmarkDest(b *testing.B) {
	var (
		field1 int
		field2 string
	)
	for i := 0; i < b.N; i++ {
		q := sqlf.From("table").
			Select("field1").To(&field1).
			Select("field2").To(&field2)
		q.Close()
	}
}

func selectComplex(b *testing.B, builder *sqlf.Builder) {
	for n := 0; n < b.N; n++ {
		q := builder.Select("DISTINCT a, b, z, y, x").
			// Distinct().
			From("c").
			Where("d = ? OR e = ?", 1, "wat").
			// Where(dbr.Eq{"f": 2, "x": "hi"}).
			Where("g = ?", 3).
			// Where(dbr.Eq{"h": []int{1, 2, 3}}).
			GroupBy("i").
			GroupBy("ii").
			GroupBy("iii").
			Having("j = k").
			Having("jj = ?", 1).
			Having("jjj = ?", 2).
			OrderBy("l").
			OrderBy("l").
			OrderBy("l").
			Limit(7).
			Offset(8)
		q.Build()
		q.Close()
	}
}

func selectSubquery(b *testing.B, builder *sqlf.Builder) {
	for n := 0; n < b.N; n++ {
		sq := builder.Select("id").
			From("tickets").
			Where("subdomain_id = ? and (state = ? or state = ?)", 1, "open", "spam")
		subQuery, _ := sq.Build()

		q := builder.Select("DITINCT a, b").
			Select(fmt.Sprintf("(%s) AS subq", subQuery)).
			From("c").
			// Distinct().
			// Where(dbr.Eq{"f": 2, "x": "hi"}).
			Where("g = ?", 3).
			OrderBy("l").
			OrderBy("l").
			Limit(7).
			Offset(8)
		q.Build()
		q.Close()
		sq.Close()
	}
}

func BenchmarkSelectComplex(b *testing.B) {
	selectComplex(b, builderNo)
}

func BenchmarkSelectComplexPg(b *testing.B) {
	selectComplex(b, builderPg)
}

func BenchmarkSelectSubquery(b *testing.B) {
	selectSubquery(b, builderNo)
}

func BenchmarkSelectSubqueryPostgreSQL(b *testing.B) {
	selectSubquery(b, builderPg)
}
