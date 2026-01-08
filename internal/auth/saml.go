package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// SAMLProvider implements AuthProvider for SAML 2.0 identity providers
type SAMLProvider struct {
	config      ProviderConfig
	db          *gorm.DB
	sp          saml.ServiceProvider
	idpMetadata *saml.EntityDescriptor
}

// NewSAMLProvider creates a new SAML authentication provider
func NewSAMLProvider(cfg ProviderConfig, db *gorm.DB) (*SAMLProvider, error) {
	// Parse the redirect URI to get the root URL
	redirectURL, err := url.Parse(cfg.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redirect URI: %w", err)
	}

	// Build root URL (scheme + host)
	rootURL := url.URL{
		Scheme: redirectURL.Scheme,
		Host:   redirectURL.Host,
	}

	// Build ACS URL (Assertion Consumer Service)
	acsURL := rootURL
	acsURL.Path = redirectURL.Path

	// Fetch or build IdP metadata
	var idpMetadata *saml.EntityDescriptor
	if cfg.IDPMetadataURL != "" {
		// Fetch metadata from URL
		idpMetadata, err = fetchIDPMetadata(cfg.IDPMetadataURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch IdP metadata: %w", err)
		}
	} else {
		// Build metadata from manual configuration
		idpMetadata, err = buildIDPMetadata(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build IdP metadata: %w", err)
		}
	}

	// Create Service Provider
	sp := saml.ServiceProvider{
		EntityID:          cfg.SPEntityID,
		AcsURL:            acsURL,
		IDPMetadata:       idpMetadata,
		AllowIDPInitiated: true,
	}

	return &SAMLProvider{
		config:      cfg,
		db:          db,
		sp:          sp,
		idpMetadata: idpMetadata,
	}, nil
}

// fetchIDPMetadata fetches SAML metadata from a URL
func fetchIDPMetadata(metadataURL string) (*saml.EntityDescriptor, error) {
	parsedURL, err := url.Parse(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("invalid metadata URL: %w", err)
	}

	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	metadata, err := samlsp.FetchMetadata(context.Background(), client, *parsedURL)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

// buildIDPMetadata builds SAML metadata from manual configuration
func buildIDPMetadata(cfg ProviderConfig) (*saml.EntityDescriptor, error) {
	// Parse the IdP certificate
	block, _ := pem.Decode([]byte(cfg.IDPCertificate))
	if block == nil {
		// Try base64 decoding if not PEM
		certBytes, err := base64.StdEncoding.DecodeString(cfg.IDPCertificate)
		if err != nil {
			return nil, fmt.Errorf("failed to decode IdP certificate: not PEM or base64")
		}
		block = &pem.Block{Bytes: certBytes}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IdP certificate: %w", err)
	}

	// Parse SSO URL
	ssoURL, err := url.Parse(cfg.IDPSSOURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IdP SSO URL: %w", err)
	}

	// Determine entity ID (use SSO URL host if not provided)
	entityID := cfg.IDPEntityID
	if entityID == "" {
		entityID = ssoURL.Scheme + "://" + ssoURL.Host
	}

	// Build minimal IdP metadata
	metadata := &saml.EntityDescriptor{
		EntityID: entityID,
		IDPSSODescriptors: []saml.IDPSSODescriptor{
			{
				SSODescriptor: saml.SSODescriptor{
					RoleDescriptor: saml.RoleDescriptor{
						KeyDescriptors: []saml.KeyDescriptor{
							{
								Use: "signing",
								KeyInfo: saml.KeyInfo{
									X509Data: saml.X509Data{
										X509Certificates: []saml.X509Certificate{
											{Data: base64.StdEncoding.EncodeToString(cert.Raw)},
										},
									},
								},
							},
						},
					},
				},
				SingleSignOnServices: []saml.Endpoint{
					{
						Binding:  saml.HTTPRedirectBinding,
						Location: cfg.IDPSSOURL,
					},
					{
						Binding:  saml.HTTPPostBinding,
						Location: cfg.IDPSSOURL,
					},
				},
			},
		},
	}

	return metadata, nil
}

// Name returns the provider identifier
func (p *SAMLProvider) Name() string {
	return "saml"
}

// GetAuthorizationURL returns the URL to redirect users for SAML login
func (p *SAMLProvider) GetAuthorizationURL(state string) (string, error) {
	// Create SAML AuthnRequest
	authReq, err := p.sp.MakeAuthenticationRequest(
		p.sp.GetSSOBindingLocation(saml.HTTPRedirectBinding),
		saml.HTTPRedirectBinding,
		saml.HTTPPostBinding,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create SAML auth request: %w", err)
	}

	// Build redirect URL with SAMLRequest and RelayState
	redirectURL, err := authReq.Redirect(state, &p.sp)
	if err != nil {
		return "", fmt.Errorf("failed to create SAML redirect URL: %w", err)
	}

	return redirectURL.String(), nil
}

// HandleCallback processes the SAML response callback
func (p *SAMLProvider) HandleCallback(ctx context.Context, req CallbackRequest) (*AuthResult, error) {
	// Decode SAMLResponse
	if req.SAMLResponse == "" {
		return nil, fmt.Errorf("no SAMLResponse in callback")
	}

	// Parse and validate the SAML response
	// ParseResponse expects the request (can be nil for IdP-initiated) and the base64-encoded SAMLResponse(s)
	assertion, err := p.sp.ParseResponse(nil, []string{req.SAMLResponse})
	if err != nil {
		return nil, fmt.Errorf("failed to validate SAML response: %w", err)
	}

	// Extract user attributes
	email := ""
	name := ""

	// Check NameID first
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		nameID := assertion.Subject.NameID.Value
		// If NameID format is email, use it
		if assertion.Subject.NameID.Format == "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" {
			email = nameID
		}
	}

	// Check attribute statements for email and name
	for _, statement := range assertion.AttributeStatements {
		for _, attr := range statement.Attributes {
			if len(attr.Values) == 0 {
				continue
			}
			value := attr.Values[0].Value

			// Check for email attributes
			switch attr.Name {
			case "email",
				"mail",
				"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
				"http://schemas.xmlsoap.org/claims/EmailAddress",
				"urn:oid:0.9.2342.19200300.100.1.3": // mail OID
				if email == "" {
					email = value
				}
			case "name",
				"displayName",
				"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
				"http://schemas.microsoft.com/identity/claims/displayname",
				"urn:oid:2.16.840.1.113730.3.1.241": // displayName OID
				if name == "" {
					name = value
				}
			case "givenName",
				"firstName",
				"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
				"urn:oid:2.5.4.42": // givenName OID
				if name == "" {
					name = value
				}
			}
		}
	}

	// Fall back to NameID for email if not found in attributes
	if email == "" && assertion.Subject != nil && assertion.Subject.NameID != nil {
		email = assertion.Subject.NameID.Value
	}

	if email == "" {
		return nil, fmt.Errorf("no email found in SAML assertion")
	}

	// Find or create user
	user, err := p.findOrCreateUser(email, name)
	if err != nil {
		return nil, err
	}

	// Use assertion ID as session identifier
	sessionID := ""
	if assertion.ID != "" {
		sessionID = assertion.ID
	}

	return &AuthResult{
		User:      user,
		SessionID: sessionID,
	}, nil
}

// GetLogoutURL returns the URL for SAML Single Logout
func (p *SAMLProvider) GetLogoutURL(sessionID string) (string, error) {
	// SAML SLO is complex and requires proper session management
	// For initial implementation, return empty to indicate local logout only
	return "", nil
}

// SupportsLogout returns whether this provider supports logout URLs
func (p *SAMLProvider) SupportsLogout() bool {
	return false
}

// findOrCreateUser finds an existing user by email or creates a new one
func (p *SAMLProvider) findOrCreateUser(email, name string) (*models.User, error) {
	var user models.User
	result := p.db.Where("email = ?", email).First(&user)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new vendor user
			user = models.User{
				Email: email,
				Name:  name,
				Role:  models.RoleVendor,
			}
			if err := p.db.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query user: %w", result.Error)
		}
	} else {
		// Update existing user's name if it was empty
		if user.Name == "" && name != "" {
			user.Name = name
			p.db.Save(&user)
		}
	}

	return &user, nil
}

// GetSPMetadataHandler returns an HTTP handler that serves the SP metadata
// This is useful for IdP configuration
func (p *SAMLProvider) GetSPMetadataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metadata := p.sp.Metadata()
		metadataBytes, err := xml.MarshalIndent(metadata, "", "  ")
		if err != nil {
			http.Error(w, "Failed to serialize metadata", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/samlmetadata+xml")
		if _, err := w.Write(metadataBytes); err != nil {
			http.Error(w, "Failed to write metadata", http.StatusInternalServerError)
		}
	}
}
