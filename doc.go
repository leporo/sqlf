// Package sqlf is an SQL statement builder and executor.
/*

SQL Statement Builder

sqlf statement builder provides a way to:
- Combine SQL statements from fragments of raw SQL and arguments that match
  those fragments,
- Map columns to variables to be referenced by Scan,
- Convert ? placeholders into numbered ones for PostgreSQL ($1, $2, etc).
*/
package sqlf
