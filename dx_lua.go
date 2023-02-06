package tail

import (
	"fmt"
	cond "github.com/vela-ssoc/vela-cond"
	"github.com/vela-ssoc/vela-kit/lua"
	"time"
)

func (dx *Dx) String() string                         { return fmt.Sprintf("%p", &dx) }
func (dx *Dx) Type() lua.LValueType                   { return lua.LTObject }
func (dx *Dx) AssertFloat64() (float64, bool)         { return 0, false }
func (dx *Dx) AssertString() (string, bool)           { return "", false }
func (dx *Dx) AssertFunction() (*lua.LFunction, bool) { return nil, false }
func (dx *Dx) Peek() lua.LValue                       { return dx }

func (dx *Dx) inotifyL(L *lua.LState) int {
	xEnv.Spawn(0, dx.inotify)
	return 0
}

func (dx *Dx) pollL(L *lua.LState) int {
	n := L.IsInt(1)
	td := time.Second * 30
	if n > 0 {
		td = time.Second * time.Duration(n)
	}

	xEnv.Spawn(0, func() {
		dx.poll(td)
	})
	return 0
}

func (dx *Dx) onL(L *lua.LState) int {
	dx.on.Check(L, 1)
	dx.co = xEnv.Clone(L)
	return 0
}

func (dx *Dx) runL(L *lua.LState) int {
	e := dx.readDir()
	if e != nil {
		L.RaiseError("%s/%s run fail %v", dx.dir, dx.base, e)
	}
	return 0
}

func (dx *Dx) filterL(L *lua.LState) int {
	if dx.cnd == nil {
		dx.cnd = cond.CheckMany(L)
	} else {
		cnd := cond.CheckMany(L)
		dx.cnd.Merge(cnd)
	}
	return 0
}

func (dx *Dx) Index(L *lua.LState, key string) lua.LValue {
	switch key {

	case "inotify":
		return L.NewFunction(dx.inotifyL)

	case "poll":
		return L.NewFunction(dx.pollL)

	case "filter":
		return L.NewFunction(dx.filterL)

	case "on":
		return L.NewFunction(dx.onL)

	case "run":
		return L.NewFunction(dx.runL)
	}

	return lua.LNil
}
