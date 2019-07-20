package sqlf_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/leporo/sqlf"
)

var builderPg = sqlf.NewBuilder(sqlf.PostgreSQL())

func BenchmarkString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		strings.Join([]string{"SELECT", "id", "FROM", "table", "WHERE", "id > ? AND id < ?"}, " ")
	}
}

func BenchmarkSelect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sqlf.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000).Build()
	}
}

func BenchmarkSelectWithClose(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := sqlf.Select("id").From("table").Where("id > ?", 42).Where("id < ?", 1000)
		q.Build()
		q.Close()
	}
}

func BenchmarkSelectWithClosePg(b *testing.B) {
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
