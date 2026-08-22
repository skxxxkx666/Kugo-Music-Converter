//go:build !windows || !amd64

package qmckey

import "context"

type localCredentialSource struct{}

func newLocalCredentialSource() credentialSource { return localCredentialSource{} }

func (localCredentialSource) Load(context.Context) (credentials, error) {
	return credentials{}, ErrUnavailable
}
