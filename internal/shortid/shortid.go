package shortid

import (
	gonanoid "github.com/matoous/go-nanoid/v2"
)

const (
	shortIDLen     = 26
	nanoIDAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	nanoIDLen      = 23
)

// New generates a new short ID with the given prefix.
// The resulting ID is 26 characters: 3-char prefix + 23-char NanoID (base36).
func New(prefix string) string {
	id, err := gonanoid.Generate(nanoIDAlphabet, nanoIDLen)
	if err != nil {
		panic(err)
	}
	return prefix + id
}

// IsValid checks if the given string is a valid short ID (26 characters).
func IsValid(id string) bool {
	return len(id) == shortIDLen
}

// Domain-specific ID constructors

func NewUserID() string {
	return New("iur")
}

func NewNuonOrgID() string {
	return New("ino")
}

func NewInstallLinkID() string {
	return New("ilk")
}

func NewInstallID() string {
	return New("isi")
}

func NewThemeID() string {
	return New("ith")
}

func NewHealthCheckConfigID() string {
	return New("ihc")
}

func NewCustomerAuthConfigID() string {
	return New("cac")
}

func NewAppInputConfigID() string {
	return New("aic")
}

func NewOrgMemberID() string {
	return New("ogm")
}

func NewOrgInvitationID() string {
	return New("ogi")
}

func NewGitHubRepoConfigID() string {
	return New("ghc")
}

func NewTemplateOverrideID() string {
	return New("tov")
}

func NewAssetOverrideID() string {
	return New("aov")
}
