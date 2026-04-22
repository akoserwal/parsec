package probe

import (
	"go.opentelemetry.io/otel/attribute"

	"github.com/project-kessel/parsec/internal/service"
)

// Attribute keys used across all metrics probes.
// All attribute values MUST be low-cardinality: every value comes from a
// closed enum below. Unbounded strings (user IDs, error messages, full URNs)
// must never appear as attribute values.
var (
	AttrKeyTokenType          = attribute.Key("token_type")
	AttrKeyStatus             = attribute.Key("status")
	AttrKeyGrantType          = attribute.Key("grant_type")
	AttrKeyRequestedTokenType = attribute.Key("requested_token_type")
	AttrKeyDatasource         = attribute.Key("datasource")
	AttrKeyCacheResult        = attribute.Key("result")
	AttrKeyProviderType       = attribute.Key("provider_type")
)

// Token type short labels. Maps service.TokenType URNs to bounded metric labels.
const (
	TokenTypeLabelTxnToken    = "txn_token"
	TokenTypeLabelAccessToken = "access_token"
	TokenTypeLabelJWT         = "jwt"
	TokenTypeLabelRHIdentity  = "rh_identity"
	TokenTypeLabelOther       = "other"
)

// TokenTypeLabel maps a service.TokenType URN to a short, bounded label.
func TokenTypeLabel(tt service.TokenType) string {
	switch tt {
	case service.TokenTypeTransactionToken:
		return TokenTypeLabelTxnToken
	case service.TokenTypeAccessToken:
		return TokenTypeLabelAccessToken
	case service.TokenTypeJWT:
		return TokenTypeLabelJWT
	case service.TokenTypeRHIdentity:
		return TokenTypeLabelRHIdentity
	default:
		return TokenTypeLabelOther
	}
}

// Status labels for operation outcomes.
const (
	StatusSuccess                    = "success"
	StatusFailure                    = "failure"
	StatusIssuerNotFound             = "issuer_not_found"
	StatusActorValidationFailed      = "actor_validation_failed"
	StatusSubjectValidationFailed    = "subject_validation_failed"
	StatusRequestContextParseFailed  = "request_context_parse_failed"
	StatusCredentialExtractionFailed = "credential_extraction_failed"
)

// Cache result labels.
const (
	CacheResultHit     = "hit"
	CacheResultMiss    = "miss"
	CacheResultExpired = "expired"
	CacheResultError   = "error"
)

// Lua fetch status labels.
const (
	LuaStatusCompleted        = "completed"
	LuaStatusCompletedNil     = "completed_nil"
	LuaStatusScriptLoadFailed = "script_load_failed"
	LuaStatusExecutionFailed  = "execution_failed"
	LuaStatusInvalidReturn    = "invalid_return"
	LuaStatusConversionFailed = "conversion_failed"
)

// Key provider type labels.
const (
	ProviderTypeKMS    = "kms"
	ProviderTypeDisk   = "disk"
	ProviderTypeMemory = "memory"
)

// Grant type labels.
const (
	GrantTypeTokenExchange = "token_exchange"
	GrantTypeOther         = "other"
)

// GrantTypeLabel maps a grant type URN to a short, bounded label.
func GrantTypeLabel(gt string) string {
	if gt == "urn:ietf:params:oauth:grant-type:token-exchange" {
		return GrantTypeTokenExchange
	}
	return GrantTypeOther
}
