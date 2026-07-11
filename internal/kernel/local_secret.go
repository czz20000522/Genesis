package kernel

import "genesis/localconfig"

var (
	ErrLocalSecretRefInvalid  = localconfig.ErrCredentialRefInvalid
	ErrLocalSecretMissing     = localconfig.ErrCredentialMissing
	ErrLocalSecretUnreadable  = localconfig.ErrCredentialUnreadable
	ErrLocalSecretUnsupported = localconfig.ErrCredentialUnsupported
)

type LocalCredentialSecretWriteRequest = localconfig.CredentialSecretWriteRequest
type LocalCredentialSecretWriteResult = localconfig.CredentialSecretWriteResult

func ResolveLocalCredentialSecret(ref string, storeRoot string) (string, error) {
	return localconfig.ResolveCredentialSecret(ref, storeRoot)
}

func WriteLocalCredentialSecret(req LocalCredentialSecretWriteRequest) (LocalCredentialSecretWriteResult, error) {
	return localconfig.WriteCredentialSecret(req)
}

func isLocalSecretCredentialRef(value string) bool {
	return localconfig.NormalizeCredentialRef(value) != ""
}

func normalizeLocalSecretRef(value string) string {
	return localconfig.NormalizeCredentialRef(value)
}

func localSecretPath(ref string, storeRoot string) string {
	return localconfig.CredentialPath(ref, storeRoot)
}
