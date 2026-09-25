package service

import "strings"

const tokenGuardSecretMask = "********"

// Public responses are copies: masking must never change credentials used by workers.
func publicTokenGuardConfig(c AccountTokenGuardConfig) AccountTokenGuardConfig {
	mask := func(v string) string {
		if v != "" {
			return tokenGuardSecretMask
		}
		return ""
	}
	headers := func(h map[string]string) map[string]string {
		out := make(map[string]string, len(h))
		for k, v := range h {
			out[k] = mask(v)
		}
		return out
	}
	c.ProbeHeaders = headers(c.ProbeHeaders)
	c.ReloginHeaders = headers(c.ReloginHeaders)
	c.BarkKey = mask(c.BarkKey)
	c.ReloginAccounts = append([]AccountTokenGuardReloginAccount(nil), c.ReloginAccounts...)
	for i := range c.ReloginAccounts {
		c.ReloginAccounts[i].Password = mask(c.ReloginAccounts[i].Password)
		c.ReloginAccounts[i].MFASecret = mask(c.ReloginAccounts[i].MFASecret)
	}
	return c
}
func restoreTokenGuardSecrets(c, previous AccountTokenGuardConfig) AccountTokenGuardConfig {
	restore := func(v, old string) string {
		if v == tokenGuardSecretMask {
			return old
		}
		return v
	}
	headers := func(h, old map[string]string) map[string]string {
		out := make(map[string]string, len(h))
		for k, v := range h {
			prior := ""
			for name, value := range old {
				if strings.EqualFold(name, k) {
					prior = value
					break
				}
			}
			out[k] = restore(v, prior)
		}
		return out
	}
	c.ProbeHeaders = headers(c.ProbeHeaders, previous.ProbeHeaders)
	c.ReloginHeaders = headers(c.ReloginHeaders, previous.ReloginHeaders)
	c.BarkKey = restore(c.BarkKey, previous.BarkKey)
	c.ReloginAccounts = append([]AccountTokenGuardReloginAccount(nil), c.ReloginAccounts...)
	for i := range c.ReloginAccounts {
		for _, old := range previous.ReloginAccounts {
			if strings.EqualFold(strings.TrimSpace(c.ReloginAccounts[i].Email), old.Email) {
				c.ReloginAccounts[i].Password = restore(c.ReloginAccounts[i].Password, old.Password)
				c.ReloginAccounts[i].MFASecret = restore(c.ReloginAccounts[i].MFASecret, old.MFASecret)
				break
			}
		}
	}
	return c
}
