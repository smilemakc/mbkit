package roles

type Role int

const (
	SimpleUser    Role = 1 << 0 // SimpleUser represents a role for basic users with the least access permission in the system. 1
	AdminRole     Role = 1 << 8 // AdminRole represents an admin user role with specific elevated permissions. 256
	SuperUserRole Role = 1 << 9 // SuperUserRole is a Role constant representing a superuser with elevated permissions. 512
)

// Has checks if the bitwise AND of Role `r` and Role `check` is non-zero, indicating `r` contains `check`.
func Has(r Role, check Role) bool {
	return r&check != 0
}

// Add combines the Role `r` with the Role `add` using a bitwise OR operation and returns the resulting Role.
func Add(r Role, add Role) Role {
	return r | add
}

// Remove clears the specified bits in the Role `r` that match the bits set in the Role `remove`.
func Remove(r Role, remove Role) Role {
	return r &^ remove
}

// ToMask combines a slice of Role values into a single Role mask using bitwise OR operations.
func ToMask(roles ...Role) Role {
	var mask Role
	for _, r := range roles {
		mask |= r
	}
	return mask
}

// HasAny checks if any role from `roles` exists in `targetRoles` slice
func HasAny(targetRoles []Role, sourceRoles ...Role) bool {
	if len(targetRoles) == 0 || len(sourceRoles) == 0 {
		return false
	}

	checkMask := ToMask(targetRoles...)
	userMask := ToMask(sourceRoles...)
	return userMask&checkMask != 0
}

func AdminOrSuperuserRoles() []Role {
	return []Role{AdminRole, SuperUserRole}
}
