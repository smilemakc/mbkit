package policy

import (
	"context"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smilemakc/mbkit/pkg/roles"
	"github.com/stretchr/testify/assert"
)

func TestWithPrincipal(t *testing.T) {
	t.Run("should add principal to context", func(t *testing.T) {
		principal := &Principal{
			UserID:   "user123",
			Roles:    []roles.Role{1, 2},
			TenantID: "tenantABC",
			Extra:    map[string]any{"key": "value"},
		}
		ctx := context.Background()
		ctxWithPrincipal := WithPrincipal(ctx, principal)

		retrievedPrincipal, ok := ctxWithPrincipal.Value(principalKey).(*Principal)
		assert.True(t, ok)
		assert.Equal(t, principal, retrievedPrincipal)
	})
}

func TestWithGinPrincipal(t *testing.T) {
	t.Run("should add principal to gin context", func(t *testing.T) {
		principal := &Principal{
			UserID:   "user123",
			Roles:    []roles.Role{1, 2},
			TenantID: "tenantABC",
			Extra:    map[string]any{"key": "value"},
		}
		ginCtx := &gin.Context{}
		WithGinPrincipal(ginCtx, principal)

		retrievedPrincipal, exists := ginCtx.Get(ginPrincipalKey)
		assert.True(t, exists)
		assert.Equal(t, principal, retrievedPrincipal)
	})
}

func TestGetPrincipal(t *testing.T) {
	tests := []struct {
		name     string
		context  context.Context
		expected *Principal
		found    bool
	}{
		{
			name: "valid principal in context",
			context: context.WithValue(
				context.Background(),
				principalKey,
				&Principal{UserID: "user123"},
			),
			expected: &Principal{UserID: "user123"},
			found:    true,
		},
		{
			name:     "no principal in context",
			context:  context.Background(),
			expected: nil,
			found:    false,
		},
		{
			name: "invalid type in context",
			context: context.WithValue(
				context.Background(),
				principalKey,
				"not a principal",
			),
			expected: nil,
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal, ok := GetPrincipal(tt.context)
			assert.Equal(t, tt.found, ok)
			assert.Equal(t, tt.expected, principal)
		})
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name         string
		principal    *Principal
		expectedRole roles.Role
		expected     bool
	}{
		{
			name:         "nil principal",
			principal:    nil,
			expectedRole: 1,
			expected:     false,
		},
		{
			name:         "role is in admin or superuser roles",
			principal:    &Principal{Roles: []roles.Role{roles.SuperUserRole}},
			expectedRole: roles.AdminRole,
			expected:     true,
		},
		{
			name: "role exists in principal's roles",
			principal: &Principal{
				Roles: []roles.Role{roles.SimpleUser},
			},
			expectedRole: roles.AdminRole,
			expected:     false,
		},
		{
			name: "role does not exist in principal's roles",
			principal: &Principal{
				Roles: []roles.Role{roles.AdminRole},
			},
			expectedRole: roles.SuperUserRole,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasRole(tt.principal, tt.expectedRole)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUserUUIDExtractor(t *testing.T) {
	type args struct {
		p *Principal
	}
	user123 := uuid.New().String()
	tests := []struct {
		name    string
		args    args
		want    uuid.UUID
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "valid principal",
			args: args{
				p: &Principal{UserID: user123},
			},
			want:    uuid.MustParse(user123),
			wantErr: assert.NoError,
		},
		{
			name: "invalid principal",
			args: args{
				p: &Principal{UserID: "not a uuid"},
			},
			want:    uuid.Nil,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserUUIDExtractor(tt.args.p)
			if !tt.wantErr(t, err, fmt.Sprintf("UserUUIDExtractor(%v)", tt.args.p)) {
				return
			}
			assert.Equalf(t, tt.want, got, "UserUUIDExtractor(%v)", tt.args.p)
		})
	}
}

func TestIsAdminExtractor(t *testing.T) {
	type args struct {
		p *Principal
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid principal",
			args: args{
				p: &Principal{Roles: []roles.Role{roles.AdminRole}},
			},
			want: true,
		},
		{
			name: "invalid principal",
			args: args{
				p: &Principal{Roles: []roles.Role{roles.SimpleUser}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, IsAdminExtractor(tt.args.p), "IsAdminExtractor(%v)", tt.args.p)
		})
	}
}
