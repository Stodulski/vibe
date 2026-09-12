package store

// ListAuditLogsSQLForTest is the audit-log listing query, so that a test can
// EXPLAIN exactly the statement this store runs rather than a paraphrase of it.
//
// It is an ordinary declaration rather than an export_test.go one only because
// the test that reads it still lives in internal/data. When that test moves
// next to this package it becomes an export_test.go entry and this file goes
// away; nothing outside a test may use it.
const ListAuditLogsSQLForTest = listAuditLogsSQL
