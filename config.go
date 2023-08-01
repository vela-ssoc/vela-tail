package tail

import (
	auxlib2 "github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
	"github.com/vela-ssoc/vela-kit/pipe"
)

type config struct {
	name   string
	limit  int
	thread int

	add func([]byte) []byte
	enc func([]byte) []byte

	co   *lua.LState
	sdk  lua.Writer
	pipe *pipe.Chains
}

func newConfig(L *lua.LState) *config {
	val := L.Get(1)
	cfg := &config{
		co:     xEnv.Clone(L),
		thread: 2,
		limit:  0,
		pipe:   pipe.New(),
	}

	switch val.Type() {
	case lua.LTString:
		cfg.name = val.String()

	case lua.LTTable:
		val.(*lua.LTable).Range(func(key string, val lua.LValue) {
			switch key {
			case "name":
				cfg.name = auxlib2.CheckProcName(val, L)
			case "limit":
				cfg.limit = lua.IsInt(val)
			case "thread":
				cfg.thread = lua.CheckInt(L, val)
			case "to":
				cfg.sdk = auxlib2.CheckWriter(val, L)

			default:
				//todo
			}
		})

	}

	return cfg
}

func (cfg *config) valid() error {
	if e := auxlib2.Name(cfg.name); e != nil {
		return e
	}
	return nil
}
