package roles

import (
	"testing"
)

func TestHas(t *testing.T) {
	r := SimpleUser | PremiumUser

	if !Has(r, SimpleUser) {
		t.Errorf("expected SimpleUser to be present")
	}
	if Has(r, CorporateUser) {
		t.Errorf("did not expect CorporateUser to be present")
	}
}

func TestAdd(t *testing.T) {
	r := SimpleUser
	r = Add(r, PremiumUser)

	if !Has(r, PremiumUser) {
		t.Errorf("expected PremiumUser to be added")
	}
}

func TestRemove(t *testing.T) {
	r := SimpleUser | PremiumUser
	r = Remove(r, PremiumUser)

	if Has(r, PremiumUser) {
		t.Errorf("expected PremiumUser to be removed")
	}
	if !Has(r, SimpleUser) {
		t.Errorf("expected SimpleUser to still be present")
	}
}

func TestToMask(t *testing.T) {
	slice := []Role{SimpleUser, PremiumUser, CorporateUser}
	mask := ToMask(slice...)

	if !Has(mask, SimpleUser) ||
		!Has(mask, PremiumUser) ||
		!Has(mask, CorporateUser) {
		t.Errorf("expected mask to contain all roles from slice")
	}
}

func TestHasAny(t *testing.T) {
	userRoles := []Role{SimpleUser, PremiumUser}

	if !HasAny(userRoles, CorporateUser, PremiumUser) {
		t.Errorf("expected HasAny to return true (PremiumUser present)")
	}

	if HasAny(userRoles, CorporateUser, AdminRole) {
		t.Errorf("expected HasAny to return false (no matching roles)")
	}

	if HasAny(nil, SimpleUser) {
		t.Errorf("expected HasAny to return false for nil slice")
	}

	if HasAny(userRoles) {
		t.Errorf("expected HasAny to return false for empty check roles")
	}

	if HasAny(AdminOrSuperuserRoles(), SimpleUser, PremiumUser) {
		t.Errorf("expected HasAny to return false for empty check roles")
	}

	if !HasAny(AdminOrSuperuserRoles(), PremiumUser, AdminRole) {
		t.Errorf("expected HasAny to return true (PremiumUser present)")
	}
}
