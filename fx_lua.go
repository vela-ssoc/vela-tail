package tail

import (
	"fmt"
	cond "github.com/vela-ssoc/vela-cond"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
	"github.com/vela-ssoc/vela-kit/pipe"
	"time"
)

func (fx *Fx) String() string                         { return fmt.Sprintf("%p", &fx) }
func (fx *Fx) Type() lua.LValueType                   { return lua.LTObject }
func (fx *Fx) AssertFloat64() (float64, bool)         { return 0, false }
func (fx *Fx) AssertString() (string, bool)           { return "", false }
func (fx *Fx) AssertFunction() (*lua.LFunction, bool) { return nil, false }
func (fx *Fx) Peek() lua.LValue                       { return fx }

func (fx *Fx) waitL(L *lua.LState) int {
	n := L.IsInt(1)
	if n <= 0 {
		fx.watcher.wait = 5 * 1000 * time.Millisecond
	} else {
		fx.watcher.wait = time.Duration(n) * time.Millisecond
	}

	return 0
}

func (fx *Fx) delimL(L *lua.LState) int {
	val := L.IsString(1)
	if len(val) > 0 {
		fx.delim = val[0]
	}
	return 0
}

func (fx *Fx) bufferL(L *lua.LState) int {
	buffer := L.IsInt(1)
	if buffer > 1 {
		fx.buffer = buffer
	}
	return 0
}

func (fx *Fx) bucketL(L *lua.LState) int {
	n := L.GetTop()
	if n == 0 {
		return 0
	}
	var bkt []string
	for i := 1; i <= n; i++ {
		name := L.IsString(i)
		if len(name) < 2 {
			xEnv.Errorf("%s seek record invalid name %s", fx.path, name)
			return 0
		}
		bkt = append(bkt, name)
	}
	fx.bkt = bkt
	return 0
}

func (fx *Fx) pipeL(L *lua.LState) int {
	fx.pipe.CheckMany(L, pipe.Seek(0))
	return 0
}

func (fx *Fx) jsonL(L *lua.LState) int {
	fx.enc = newJson(L.Get(1))
	return 0
}

func (fx *Fx) nodeL(L *lua.LState) int {
	codec := L.CheckString(1)
	switch codec {
	case "json":
		tab := L.CreateTable(0, 2)
		tab.RawSetString("id", lua.S2L(xEnv.ID()))
		tab.RawSetString("inet", lua.S2L(xEnv.Inet()))
		tab.RawSetString("file", lua.S2L(fx.path))
		fx.enc = newJson(tab)

	case "raw":
		fx.enc = func(v []byte) []byte {
			v = append(v, ' ')
			v = append(v, auxlib.S2B(xEnv.ID())...)
			v = append(v, ' ')
			v = append(v, auxlib.S2B(xEnv.Inet())...)
			v = append(v, ' ')
			v = append(v, auxlib.S2B(fx.path)...)
			return v
		}

	default:
		//todo
	}

	return 0
}

func (fx *Fx) addL(L *lua.LState) int {
	codec := L.CheckString(1)
	switch codec {
	case "json":
		fx.add = newAddJson(L.Get(2))
		return 0
	case "raw":
		fx.add = newAddRaw(L.Get(2))
		return 0
	}
	return 0
}

func (fx *Fx) runL(L *lua.LState) int {
	xEnv.Spawn(0, func() {
		if e := fx.open(); e != nil {
			xEnv.Errorf("%s file open error %v", fx.path, e)
			return
		}
		xEnv.Errorf("%s file open succeed", fx.path)
	})
	return 0
}

func (fx *Fx) onL(L *lua.LState) int {
	fx.watcher.onEOF.Check(L, 1)
	return 0
}

func (fx *Fx) filterL(L *lua.LState) int {
	if fx.cnd == nil {
		fx.cnd = cond.CheckMany(L)
	} else {
		cnd := cond.CheckMany(L)
		fx.cnd.Merge(cnd)
	}
	return 0
}

func (fx *Fx) pollL(L *lua.LState) int {
	interval := 5 * time.Second
	n := L.IsInt(1)
	if n > 0 {
		interval = time.Duration(n) * time.Millisecond
	}

	c := L.IsInt(2)
	timeout := 24 * 365 * 20 * time.Hour //20 year
	if c > 0 {
		timeout = time.Second * time.Duration(c)
	}

	fx.watcher.mode = WTPoll
	fx.watcher.interval = interval
	fx.watcher.timeout = timeout
	return 0
}

func (fx *Fx) inotifyL(L *lua.LState) int {

	n := L.IsInt(1)
	timeout := 24 * 365 * 20 * time.Hour //20 year
	if n > 0 {
		timeout = time.Duration(n) * time.Second
	}

	fx.watcher.mode = WTInotify
	fx.watcher.timeout = timeout
	return 0
}

func (fx *Fx) afterL(L *lua.LState) int {
	n := L.IsInt(1)
	if n <= 0 {
		return 0
	}

	fx.watcher.after = time.Duration(n) * time.Second
	return 0
}

func (fx *Fx) Index(L *lua.LState, key string) lua.LValue {

	if fx.co == nil {
		fx.co = xEnv.Clone(L)
	}

	switch key {

	case "json":
		return L.NewFunction(fx.jsonL)

	case "add":
		return L.NewFunction(fx.addL)

	case "filter":
		return L.NewFunction(fx.filterL)

	case "wait":
		return L.NewFunction(fx.waitL)

	case "delim":
		return L.NewFunction(fx.delimL)

	case "buffer":
		return L.NewFunction(fx.bufferL)

	case "bkt":
		return L.NewFunction(fx.bucketL)

	case "node":
		return L.NewFunction(fx.nodeL)

	case "pipe":
		return L.NewFunction(fx.pipeL)

	case "run":
		return L.NewFunction(fx.runL)

	case "on":
		return L.NewFunction(fx.onL)

	case "path":
		return lua.S2L(fx.path)

	case "after":
		return lua.NewFunction(fx.afterL)

	case "poll":
		return lua.NewFunction(fx.pollL)

	case "inotify":
		return lua.NewFunction(fx.inotifyL)

	}

	return lua.LNil
}
