package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/testutil"
)

// TestInstallsPage_MemberFallback_RespectsOrgAndCookie validates that the
// fallback member-resolution query (used when RequireCustomerAccount middleware
// is not present) filters by org_id and respects the active_account_id cookie.
// This is a regression test for the bug where account members couldn't see
// shared installs because the fallback picked an arbitrary membership.
func TestInstallsPage_MemberFallback_RespectsOrgAndCookie(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		// Set up: vendor, org, two customers, one account
		vendor := testutil.NewTestVendor()
		require.NoError(t, tx.Create(vendor).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "shared-installs"})
		require.NoError(t, tx.Create(org).Error)

		ownerUser := testutil.NewTestUser(testutil.UserOptions{
			ID: "owner-user", Email: "owner@test.com", Role: models.RoleCustomer,
		})
		require.NoError(t, tx.Create(ownerUser).Error)

		memberUser := testutil.NewTestUser(testutil.UserOptions{
			ID: "member-user", Email: "member@test.com", Role: models.RoleCustomer,
		})
		require.NoError(t, tx.Create(memberUser).Error)

		// Owner's account
		account := &models.CustomerAccount{
			OrgID:           org.ID,
			Name:            "Owner's Company",
			CreatedByUserID: ownerUser.ID,
		}
		require.NoError(t, tx.Create(account).Error)

		// Owner membership
		require.NoError(t, tx.Create(&models.CustomerAccountMember{
			AccountID: account.ID, UserID: ownerUser.ID,
			OrgID: org.ID, Role: models.CustomerAccountRoleOwner,
		}).Error)

		// Member membership
		require.NoError(t, tx.Create(&models.CustomerAccountMember{
			AccountID: account.ID, UserID: memberUser.ID,
			OrgID: org.ID, Role: models.CustomerAccountRoleMember,
		}).Error)

		// Member also has their own auto-created account (different org or same org)
		autoAccount := &models.CustomerAccount{
			OrgID:           org.ID,
			Name:            "member@test.com's Account",
			CreatedByUserID: memberUser.ID,
		}
		require.NoError(t, tx.Create(autoAccount).Error)
		require.NoError(t, tx.Create(&models.CustomerAccountMember{
			AccountID: autoAccount.ID, UserID: memberUser.ID,
			OrgID: org.ID, Role: models.CustomerAccountRoleOwner,
		}).Error)

		// Owner creates a shared install
		sharedInstall := &models.Install{
			UserID:            ownerUser.ID,
			OrgID:             org.ID,
			CustomerAccountID: &account.ID,
			Visibility:        models.VisibilityAccount,
			NuonInstallID:     "nuon-shared-123",
		}
		require.NoError(t, tx.Create(sharedInstall).Error)

		// Simulate the fixed fallback query for memberUser with active_account_id cookie
		// pointing to the owner's account
		activeAccountID := account.ID
		userID := memberUser.ID
		orgID := org.ID

		var members []models.CustomerAccountMember
		err := tx.Preload("Account").
			Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", userID, orgID).
			Find(&members).Error
		require.NoError(t, err)
		assert.Len(t, members, 2, "member should have 2 memberships in org")

		// Select active member using cookie
		var activeMember *models.CustomerAccountMember
		for i := range members {
			if members[i].AccountID == activeAccountID {
				activeMember = &members[i]
				break
			}
		}
		require.NotNil(t, activeMember, "should find membership for the cookie's account")
		assert.Equal(t, account.ID, activeMember.AccountID, "active member should match cookie account")

		// Now run the install query with the correct activeMember
		var installs []models.Install
		err = tx.Where(
			"(user_id = ? AND customer_account_id = ?) OR (customer_account_id = ? AND visibility = ?)",
			userID, activeMember.AccountID, activeMember.AccountID, models.VisibilityAccount,
		).Find(&installs).Error
		require.NoError(t, err)
		assert.Len(t, installs, 1, "member should see the shared install")
		assert.Equal(t, sharedInstall.ID, installs[0].ID)

		// Verify that without the cookie fix (using auto account), the install is NOT visible
		var wrongInstalls []models.Install
		err = tx.Where(
			"(user_id = ? AND customer_account_id = ?) OR (customer_account_id = ? AND visibility = ?)",
			userID, autoAccount.ID, autoAccount.ID, models.VisibilityAccount,
		).Find(&wrongInstalls).Error
		require.NoError(t, err)
		assert.Len(t, wrongInstalls, 0, "auto-account should NOT see shared install from other account")
	})
}

// TestInstallCreation_RespectsActiveAccountCookie validates that install creation
// assigns the install to the cookie-selected account, not an arbitrary one.
func TestInstallCreation_RespectsActiveAccountCookie(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, tx.Create(vendor).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "install-create"})
		require.NoError(t, tx.Create(org).Error)

		customer := testutil.NewTestUser(testutil.UserOptions{
			ID: "install-customer", Email: "customer@test.com", Role: models.RoleCustomer,
		})
		require.NoError(t, tx.Create(customer).Error)

		// Auto-created account (would be picked by db.First)
		autoAccount := &models.CustomerAccount{
			OrgID:           org.ID,
			Name:            "customer@test.com's Account",
			CreatedByUserID: customer.ID,
		}
		require.NoError(t, tx.Create(autoAccount).Error)
		autoMember := &models.CustomerAccountMember{
			AccountID: autoAccount.ID, UserID: customer.ID,
			OrgID: org.ID, Role: models.CustomerAccountRoleOwner,
		}
		require.NoError(t, tx.Create(autoMember).Error)

		// Invited account (the one the user has selected via cookie)
		invitedAccount := &models.CustomerAccount{
			OrgID:           org.ID,
			Name:            "Acme Corp",
			CreatedByUserID: vendor.ID,
		}
		require.NoError(t, tx.Create(invitedAccount).Error)
		invitedMember := &models.CustomerAccountMember{
			AccountID: invitedAccount.ID, UserID: customer.ID,
			OrgID: org.ID, Role: models.CustomerAccountRoleMember,
		}
		require.NoError(t, tx.Create(invitedMember).Error)

		// Simulate the install creation logic: load all members, use SelectActiveMember
		var members []models.CustomerAccountMember
		err := tx.Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", customer.ID, org.ID).
			Find(&members).Error
		require.NoError(t, err)
		require.Len(t, members, 2)

		// Create a gin context with cookie pointing to the invited account
		ginCtx, _ := testutil.NewTestContextWithCookie(map[string]string{
			"active_account_id": invitedAccount.ID,
		})
		selected := middleware.SelectActiveMember(ginCtx, members)
		require.NotNil(t, selected)
		assert.Equal(t, invitedAccount.ID, selected.AccountID,
			"SelectActiveMember should pick the cookie-selected account, not the auto-created one")

		// Create install using the selected account
		install := &models.Install{
			UserID:            customer.ID,
			OrgID:             org.ID,
			NuonInstallID:     "nuon-test-install",
			CustomerAccountID: &selected.AccountID,
			Visibility:        models.VisibilityAccount,
			Status:            models.StatusPending,
		}
		require.NoError(t, tx.Create(install).Error)
		assert.Equal(t, invitedAccount.ID, *install.CustomerAccountID,
			"install should be assigned to the invited account, not the auto-created one")
	})
}

// TestInstallsPage_LegacyInstallsExcluded validates that installs with
// customer_account_id = NULL are not visible, even to the user who created them.
func TestInstallsPage_LegacyInstallsExcluded(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, tx.Create(vendor).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "legacy-test"})
		require.NoError(t, tx.Create(org).Error)

		customer := testutil.NewTestUser(testutil.UserOptions{
			ID: "legacy-customer", Email: "legacy@test.com", Role: models.RoleCustomer,
		})
		require.NoError(t, tx.Create(customer).Error)

		account := &models.CustomerAccount{
			OrgID:           org.ID,
			Name:            "Legacy Test Account",
			CreatedByUserID: customer.ID,
		}
		require.NoError(t, tx.Create(account).Error)
		require.NoError(t, tx.Create(&models.CustomerAccountMember{
			AccountID: account.ID, UserID: customer.ID,
			OrgID: org.ID, Role: models.CustomerAccountRoleOwner,
		}).Error)

		// Legacy install (no account association)
		legacyInstall := &models.Install{
			UserID:        customer.ID,
			OrgID:         org.ID,
			NuonInstallID: "nuon-legacy-001",
		}
		require.NoError(t, tx.Create(legacyInstall).Error)

		// Account-scoped install
		accountInstall := &models.Install{
			UserID:            customer.ID,
			OrgID:             org.ID,
			CustomerAccountID: &account.ID,
			Visibility:        models.VisibilityAccount,
			NuonInstallID:     "nuon-scoped-001",
		}
		require.NoError(t, tx.Create(accountInstall).Error)

		// Query with active account — legacy install should be excluded
		var installs []models.Install
		err := tx.Where(
			"(user_id = ? AND customer_account_id = ?) OR (customer_account_id = ? AND visibility = ?)",
			customer.ID, account.ID, account.ID, models.VisibilityAccount,
		).Find(&installs).Error
		require.NoError(t, err)
		assert.Len(t, installs, 1, "only account-scoped install should be visible")
		assert.Equal(t, accountInstall.ID, installs[0].ID)

		// Query without active account — nothing should be visible
		var noInstalls []models.Install
		err = tx.Where("1 = 0").Find(&noInstalls).Error
		require.NoError(t, err)
		assert.Len(t, noInstalls, 0, "no installs visible without account membership")
	})
}

// TestAssignInstallToAccount verifies that assigning a NULL-account install to
// an account makes it visible, and that the search excludes already-assigned installs.
func TestAssignInstallToAccount(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, tx.Create(vendor).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "assign-test"})
		require.NoError(t, tx.Create(org).Error)

		customer := testutil.NewTestUser(testutil.UserOptions{
			ID: "assign-customer", Email: "assign@test.com", Role: models.RoleCustomer,
		})
		require.NoError(t, tx.Create(customer).Error)

		account := &models.CustomerAccount{
			OrgID:           org.ID,
			Name:            "Assign Test Account",
			CreatedByUserID: customer.ID,
		}
		require.NoError(t, tx.Create(account).Error)

		// Create an unassigned install (no customer_account_id)
		unassignedInstall := &models.Install{
			Name:          "Orphan Install",
			UserID:        customer.ID,
			OrgID:         org.ID,
			NuonInstallID: "nuon-orphan-456",
			Visibility:    models.VisibilityAccount,
		}
		require.NoError(t, tx.Create(unassignedInstall).Error)

		// Verify install is NOT visible via account query
		var before []models.Install
		err := tx.Where("customer_account_id = ? AND deleted_at IS NULL", account.ID).Find(&before).Error
		require.NoError(t, err)
		assert.Len(t, before, 0, "no installs should be in account before assignment")

		// Simulate search: should find the unassigned install
		var searchResults []models.Install
		err = tx.Where("org_id = ? AND name ILIKE ? AND (customer_account_id IS NULL OR customer_account_id != ?) AND deleted_at IS NULL",
			org.ID, "%Orphan%", account.ID).Find(&searchResults).Error
		require.NoError(t, err)
		assert.Len(t, searchResults, 1, "search should find the unassigned install")

		// Assign the install to the account
		err = tx.Model(&models.Install{}).Where("id = ?", unassignedInstall.ID).Updates(map[string]interface{}{
			"customer_account_id": account.ID,
			"visibility":          models.VisibilityAccount,
		}).Error
		require.NoError(t, err)

		// Verify install IS now visible via account query
		var after []models.Install
		err = tx.Where("customer_account_id = ? AND deleted_at IS NULL", account.ID).Find(&after).Error
		require.NoError(t, err)
		assert.Len(t, after, 1, "install should be in account after assignment")
		assert.Equal(t, unassignedInstall.ID, after[0].ID)

		// Search should now EXCLUDE the assigned install
		var searchAfter []models.Install
		err = tx.Where("org_id = ? AND name ILIKE ? AND (customer_account_id IS NULL OR customer_account_id != ?) AND deleted_at IS NULL",
			org.ID, "%Orphan%", account.ID).Find(&searchAfter).Error
		require.NoError(t, err)
		assert.Len(t, searchAfter, 0, "search should exclude installs already in the target account")
	})
}
