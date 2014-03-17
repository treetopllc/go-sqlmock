package sqlmock

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

type conn struct {
	expectations []expectation
	active       expectation
}

// Close a mock database driver connection. It should
// be always called to ensure that all expectations
// were met successfully. Returns error if there is any
func (c *conn) Close() (err error) {
	msgs := make([]string, 0, 10)
	for _, e := range mock.conn.expectations {
		if e.fulfilled() {
			var msg string
			switch ev := e.(type) {
			case *expectedExec:
				msg = fmt.Sprintf("execed \"%s\"", ev.sqlRegex.String())
			case *expectedQuery:
				msg = fmt.Sprintf("queried \"%s\"", ev.sqlRegex.String())
			}
			msgs = append(msgs, msg)
		} else {
			errs := strings.Join(msgs, "\n")
			switch ev := e.(type) {
			case *expectedExec:
				err = fmt.Errorf("%s\nthere is a remaining expectation %T, \"%s\" which was not matched yet", errs, ev, ev.sqlRegex.String())
			case *expectedQuery:
				err = fmt.Errorf("%s\nthere is a remaining expectation %T, \"%s\" which was not matched yet", errs, ev, ev.sqlRegex.String())
			default:
				err = fmt.Errorf("%s\nthere is a remaining expectation %T which was not matched yet", errs, e)
			}
			break
		}
	}
	mock.conn.expectations = []expectation{}
	mock.conn.active = nil
	return err
}

func (c *conn) Begin() (driver.Tx, error) {
	e := c.next()
	if e == nil {
		return nil, fmt.Errorf("all expectations were already fulfilled, call to begin transaction was not expected")
	}

	etb, ok := e.(*expectedBegin)
	if !ok {
		return nil, fmt.Errorf("call to begin transaction, was not expected, next expectation is %T as %+v", e, e)
	}
	etb.triggered = true
	return &transaction{c}, etb.err
}

// get next unfulfilled expectation
func (c *conn) next() (e expectation) {
	for _, e = range c.expectations {
		if !e.fulfilled() {
			return
		}
	}
	return nil // all expectations were fulfilled
}

func (c *conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	e := c.next()
	query = stripQuery(query)
	if e == nil {
		return nil, fmt.Errorf("all expectations were already fulfilled, call to exec '%s' query with args %+v was not expected", query, args)
	}

	eq, ok := e.(*expectedExec)
	if !ok {
		return nil, fmt.Errorf("call to exec query '%s' with args %+v, was not expected, next expectation is %T as %+v", query, args, e, e)
	}

	eq.triggered = true
	if eq.err != nil {
		return nil, eq.err // mocked to return error
	}

	if eq.result == nil {
		return nil, fmt.Errorf("exec query '%s' with args %+v, must return a database/sql/driver.result, but it was not set for expectation %T as %+v", query, args, eq, eq)
	}

	if !eq.queryMatches(query) {
		return nil, fmt.Errorf("exec query '%s', does not match regex '%s'", query, eq.sqlRegex.String())
	}

	if !eq.argsMatches(args) {
		return nil, fmt.Errorf("exec query '%s', args %+v does not match expected %+v", query, args, eq.args)
	}

	return eq.result, nil
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return &statement{mock.conn, stripQuery(query)}, nil
}

func (c *conn) Query(query string, args []driver.Value) (driver.Rows, error) {
	e := c.next()
	query = stripQuery(query)
	if e == nil {
		return nil, fmt.Errorf("all expectations were already fulfilled, call to query '%s' with args %+v was not expected", query, args)
	}

	eq, ok := e.(*expectedQuery)
	if !ok {
		return nil, fmt.Errorf("call to query '%s' with args %+v, was not expected, next expectation is %T as %+v", query, args, e, e)
	}

	eq.triggered = true
	if eq.err != nil {
		return nil, eq.err // mocked to return error
	}

	if eq.rows == nil {
		return nil, fmt.Errorf("query '%s' with args %+v, must return a database/sql/driver.rows, but it was not set for expectation %T as %+v", query, args, eq, eq)
	}

	if !eq.queryMatches(query) {
		return nil, fmt.Errorf("query '%s', does not match regex [%s]", query, eq.sqlRegex.String())
	}

	if !eq.argsMatches(args) {
		return nil, fmt.Errorf("query '%s', args %+v does not match expected %+v", query, args, eq.args)
	}

	return eq.rows, nil
}
