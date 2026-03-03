package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomerAccountInvite_IsUsed(t *testing.T) {
	t.Run("not used", func(t *testing.T) {
		invite := &CustomerAccountInvite{}
		assert.False(t, invite.IsUsed())
	})

	t.Run("used", func(t *testing.T) {
		userID := "test-user-id"
		invite := &CustomerAccountInvite{UsedByUserID: &userID}
		assert.True(t, invite.IsUsed())
	})
}

func TestInstallVisibility_Constants(t *testing.T) {
	assert.Equal(t, InstallVisibility("account"), VisibilityAccount)
	assert.Equal(t, InstallVisibility("private"), VisibilityPrivate)
}
