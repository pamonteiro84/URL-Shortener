package migrations

import "github.com/go-gormigrate/gormigrate/v2"

var All = []*gormigrate.Migration{
      CreateURLsTable,
      CreateUsersTable,
      AddUserIDToURLs,
}