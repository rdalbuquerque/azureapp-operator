package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/microsoft/go-mssqldb/azuread"
)

type SqlClient struct {
	*sql.DB
	context context.Context
}

func NewServicePrincipalClient(appid, appsecret, sv, db string, ctx context.Context) (*SqlClient, error) {
	connString := fmt.Sprintf("sqlserver://%s:%s@%s.database.windows.net?database=%s&fedauth=ActiveDirectoryServicePrincipal", appid, appsecret, sv, db)
	dbconn, err := sql.Open(azuread.DriverName, connString)
	if err != nil {
		return nil, err
	}
	return &SqlClient{
		DB:      dbconn,
		context: ctx,
	}, nil
}

func (c *SqlClient) CreateUser(username string) error {
	var err error

	// Check if database is alive.
	err = c.PingContext(c.context)
	if err != nil {
		return err
	}

	createUserTsql := fmt.Sprintf(`
		IF NOT EXISTS(SELECT principal_id FROM sys.database_principals WHERE name = '%s') BEGIN
			CREATE USER [%s] FROM EXTERNAL PROVIDER;
		END
	`, username, username)
	stmt, err := c.Prepare(createUserTsql)
	if err != nil {
		return err
	}
	defer stmt.Close()
	if _, err := stmt.ExecContext(c.context, username); err != nil {
		return err
	}

	return nil
}

func (c *SqlClient) GrantOwner(username string) error {
	// Check if database is alive.
	err := c.PingContext(c.context)
	if err != nil {
		return err
	}

	grantOwnerTsql := fmt.Sprintf("EXEC sp_addrolemember 'db_owner', [%s];", username)
	stmt, err := c.Prepare(grantOwnerTsql)
	if err != nil {
		return err
	}
	defer stmt.Close()
	if _, err := stmt.ExecContext(c.context, username); err != nil {
		return err
	}

	return nil
}
