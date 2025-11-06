package policy

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func principalCtx(base context.Context, userID string) context.Context {
	return context.WithValue(base, principalKey, &Principal{UserID: userID})
}

// --- Tests ---

func TestOwnerByJoinPolicy_alias(t *testing.T) {
	p1 := OwnerByJoinPolicy[string, string]{Path: RelationPath{Alias: "custom_alias"}}
	p2 := OwnerByJoinPolicy[string, string]{Path: RelationPath{Alias: ""}}

	assert.Equal(t, "custom_alias", p1.alias())
	assert.Equal(t, "t", p2.alias())
}

func TestOwnerByJoinPolicy_getCurrentUserID(t *testing.T) {
	ctx := context.Background()
	p := OwnerByJoinPolicy[string, string]{
		ParseUserID: func(s string) (string, error) { return "parsed_" + s, nil },
	}

	// Case: Unauthorized
	unauthCtx := context.WithValue(ctx, "parsed_user1", nil)
	_, err := p.getCurrentUserID(unauthCtx)
	assert.Equal(t, ErrUnauthorized, err)

	// Case: Valid user
	authCtx := principalCtx(ctx, "user1")
	result, err := p.getCurrentUserID(authCtx)
	assert.NoError(t, err)
	assert.Equal(t, "parsed_user1", result)
}

func TestOwnerByJoinPolicy_Check(t *testing.T) {
	ctx := context.Background()
	bunDB, mock, cleanup := newMockBunDB(t)
	defer cleanup()

	p := OwnerByJoinPolicy[string, string]{
		Path: RelationPath{
			FromTable: "test_table",
			OwnerExpr: "test_table.owner_id",
		},
		ParseUserID: func(s string) (string, error) { return "parsed_" + s, nil },
	}

	// Unauthorized
	err := p.Check(context.WithValue(ctx, "principal", nil), bunDB, "", ObjID[string]{Val: "id1"})
	assert.Equal(t, ErrUnauthorized, err)

	// Exists = true
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM "test_table" WHERE \(test_table.owner_id = 'parsed_user1'\) LIMIT 1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	err = p.Check(principalCtx(ctx, "user1"), bunDB, "", ObjID[string]{Val: "id1"})
	assert.NoError(t, err)

	// Exists = false
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM "test_table" WHERE \(test_table.owner_id = 'parsed_user1'\) LIMIT 1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err = p.Check(principalCtx(ctx, "user1"), bunDB, "", ObjID[string]{Val: "id1"})
	assert.Equal(t, ErrNotFound, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOwnerByJoinPolicy_Scope(t *testing.T) {
	ctx := context.Background()
	bunDB, _, cleanup := newMockBunDB(t)
	defer cleanup()

	p := OwnerByJoinPolicy[string, string]{
		Path: RelationPath{
			FromTable: "test_table",
			OwnerExpr: "test_table.owner_id",
		},
		ParseUserID: func(s string) (string, error) { return "parsed_" + s, nil },
	}

	// Unauthorized — должен добавить Where("1=0")
	query := bunDB.NewSelect().Table("test_table")
	result := p.Scope(context.WithValue(ctx, "principal", nil), bunDB, query)
	assert.Contains(t, result.String(), "1=0")

	// Authorized — обычный scope
	authCtx := principalCtx(ctx, "user1")
	result = p.Scope(authCtx, bunDB, bunDB.NewSelect().Table("test_table"))
	assert.Contains(t, result.String(), "test_table.owner_id")
}

func TestOwnerByJoinPolicy_Validate(t *testing.T) {
	ctx := context.Background()
	bunDB, mock, cleanup := newMockBunDB(t)
	defer cleanup()

	p := OwnerByJoinPolicy[string, string]{
		ParseUserID: func(s string) (string, error) { return "parsed_" + s, nil },
		ExtractID: func(payload any) (string, bool) {
			if val, ok := payload.(string); ok {
				return val, true
			}
			return "", false
		},
		Path: RelationPath{
			FromTable: "test_table",
			OwnerExpr: "test_table.owner_id",
		},
	}

	// Unsupported action
	err := p.Validate(ctx, bunDB, "Unsupported", "payload")
	assert.NoError(t, err)

	// Missing ID
	err = p.Validate(ctx, bunDB, ActCreate, nil)
	assert.NoError(t, err)

	// Valid ID but record not found
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM "test_table" WHERE \(test_table.owner_id = 'parsed_user1'\) LIMIT 1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err = p.Validate(principalCtx(ctx, "user1"), bunDB, ActCreate, "payload")
	assert.Error(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}
