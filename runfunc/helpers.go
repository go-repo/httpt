package runfunc

import (
	"errors"
	"fmt"
	"net/http"
)

type BasicAuth struct {
	Username string
	Password string
}

type GetFuncOption struct {
	BasicAuth *BasicAuth
}

func GetFunc(url string, option *GetFuncOption) Func {
	return func(p Param) error {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		if option != nil {
			if option.BasicAuth != nil {
				req.SetBasicAuth(option.BasicAuth.Username, option.BasicAuth.Password)
			}
		}

		res, err := p.Request(req, nil)
		if err != nil {
			return err
		}

		if res.Resp().StatusCode != http.StatusOK {
			p.AddErrorMetric(errors.New(fmt.Sprintf("status code is not 200, actual is %v", res.Resp().StatusCode)), url)
			return nil
		}

		return nil
	}
}
