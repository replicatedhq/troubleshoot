package convert

import (
	"bytes"
	"strconv"
	"sync"
	"text/template"
)

var (
	funcMap   = template.FuncMap{}
	funcMapMu sync.Mutex
)

func String(text string, data interface{}) (string, error) {
	return Execute(text, data)
}

func Bool(text string, data interface{}) (bool, error) {
	str, err := Execute(text, data)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(str)
}

func Execute(text string, data interface{}) (string, error) {
	delims := []struct {
		ldelim string
		rdelim string
	}{
		{"{{repl", "}}"},
		{"repl{{", "}}"},
	}

	curText := text
	for _, d := range delims {
		tmpl, err := template.New(curText).
			Delims(d.ldelim, d.rdelim).
			Funcs(funcMap).
			Parse(curText)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		err = func() (err error) {
			defer errRecover(&err)
			err = tmpl.Execute(&buf, data)
			return
		}()
		if err != nil {
			return "", err
		}
		curText = buf.String()
	}
	return curText, nil
}

func RegisterFunc(key string, fn interface{}) {
	funcMapMu.Lock()
	funcMap[key] = fn
	funcMapMu.Unlock()
}

// errRecover is the handler that turns panics into returns from FuncMaps.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case FuncError:
			*errp = err // Keep the wrapper.
		case error:
			*errp = err // Catch panics from template functions
		default:
			panic(e)
		}
	}
}
