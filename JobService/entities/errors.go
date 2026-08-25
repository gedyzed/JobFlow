package entities

type SafeError struct {
    Code     string
    UserMsg  string
    Internal error
    Metadata map[string]any
}

func (e *SafeError) Error() string {
	return e.UserMsg
}

func (e *SafeError) Unwrap() error {
    return e.Internal
}

func (e *SafeError) Response() map[string]string {
    return map[string]string{
        "code":    e.Code,
        "userMsg": e.UserMsg,
    }
}

