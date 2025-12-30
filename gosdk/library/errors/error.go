package errors

type SDKError string

func (e SDKError) Error() string {
	return string(e)
}
