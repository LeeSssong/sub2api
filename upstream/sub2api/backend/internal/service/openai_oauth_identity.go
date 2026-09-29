package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// OpenAIOAuthStoredIdentity fills missing legacy identity fields from stored
// OAuth tokens without mutating the account. Expiration is intentionally ignored
// for matching old credentials during import and re-login. Decoding claims is
// not signature verification and must not be used to authorize a request.
func OpenAIOAuthStoredIdentity(account *Account) *openai.UserInfo {
	identity := &openai.UserInfo{}
	if account == nil {
		return identity
	}
	identity.Email = strings.TrimSpace(account.GetCredential("email"))
	identity.ChatGPTAccountID = strings.TrimSpace(account.GetCredential("chatgpt_account_id"))
	identity.ChatGPTUserID = strings.TrimSpace(account.GetCredential("chatgpt_user_id"))
	for _, key := range []string{"access_token", "id_token"} {
		claims, err := openai.DecodeIDToken(strings.TrimSpace(account.GetCredential(key)))
		if err != nil {
			continue
		}
		info := claims.GetUserInfo()
		if identity.Email == "" {
			identity.Email = strings.TrimSpace(info.Email)
		}
		if identity.ChatGPTAccountID == "" {
			identity.ChatGPTAccountID = strings.TrimSpace(info.ChatGPTAccountID)
		}
		if identity.ChatGPTUserID == "" {
			identity.ChatGPTUserID = strings.TrimSpace(info.ChatGPTUserID)
		}
	}
	return identity
}
