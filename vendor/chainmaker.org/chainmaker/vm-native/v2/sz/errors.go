package sz

import (
	"errors"
	"fmt"
)

var (
	ErrParamsEmpty = errors.New("the params is empty")
	ErrParams      = errors.New("params is error")
)

func ErrSomeParamEmpty(paramName string) string {
	s := fmt.Sprintf("%s is empty", paramName)
	return s
}
