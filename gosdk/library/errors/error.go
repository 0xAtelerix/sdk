// Package errors preserves the public SDKError import path used by downstream repos.
//
//revive:disable:package-naming
package errors

type SDKError string

func (e SDKError) Error() string {
	return string(e)
}
