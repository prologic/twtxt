package passwords

// Passwords is an interface for creating and verifying secure passwords
// An implementation must implement all methods and it is up to the impl
// which underlying crypto to use for hasing cleartext passwrods.
type Passwords interface {
	CreatePassword(password string) (string, error)
	CheckPassword(hash, password string) error
}
