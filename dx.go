package tail

import (
	"fmt"
	cond "github.com/vela-ssoc/vela-cond"
	"github.com/vela-ssoc/vela-kit/audit"
	"github.com/vela-ssoc/vela-kit/grep"
	"github.com/vela-ssoc/vela-kit/lua"
	"github.com/vela-ssoc/vela-kit/pipe"
	"gopkg.in/tomb.v2"
	"io/ioutil"
	"path/filepath"
	"sync"
)

type Dx struct {
	dir  string
	base string

	err error

	on  *pipe.Chains
	co  *lua.LState
	cnd *cond.Cond

	mu sync.Mutex
	cu map[string]*Fx

	//处理中心
	tom   *tomb.Tomb
	newFx func(string) *Fx
	match func(string) bool
}

func (dx *Dx) compile(base string) func(string) bool {
	if dx.match != nil {
		return dx.match
	}

	m := func(_ string) bool { return true }
	if base == "*" {
		return m
	}

	g, err := grep.Compile(base, nil)
	if err != nil {
		m = func(_ string) bool { return false }
	} else {
		m = g.Match
	}

	dx.match = m

	return m
}

func (dx *Dx) Match(name string) bool {
	if name == "." || name == ".." {
		return false
	}

	match := dx.compile(dx.base)
	return match(name)
}

func (dx *Dx) onPipe(fx *Fx) {
	dx.on.Do(fx, dx.co, func(err error) {
		audit.Errorf("tail %s on call pipe error %v", dx.dir, err).
			From(dx.co.CodeVM()).High().Put()
	})
}

func (dx *Dx) readDir() error {
	dx.mu.Lock()
	defer dx.mu.Unlock()

	ds, err := ioutil.ReadDir(dx.dir)
	if err != nil {
		return fmt.Errorf("%s not dir error %v", dx.dir, err)
	}

	cu := make(map[string]*Fx, len(ds))

	n := len(ds)
	for i := 0; i < n; i++ {
		name := ds[i].Name()
		if !dx.Match(name) {
			continue
		}

		path := filepath.Join(dx.dir, name)
		fx, ok := dx.cu[path]
		if ok {
			cu[path] = fx
			delete(dx.cu, path)
			continue
		}

		fx = dx.newFx(path)
		dx.onPipe(fx)
		cu[path] = fx
		xEnv.Infof("%s/%s dx new %s file succeed", dx.dir, dx.base, path)
	}

	//清除不存在
	for _, fx := range dx.cu {
		fx.exit()
		xEnv.Errorf("%s/%s dx clean %s", dx.dir, dx.base, fx.path)
	}

	//缓存现在
	dx.cu = cu
	return nil
}

func newDx(dir, base string, tom *tomb.Tomb, newFx func(string) *Fx) *Dx {
	return &Dx{
		tom:   tom,
		base:  base,
		newFx: newFx,
		on:    pipe.New(),
		cu:    make(map[string]*Fx, 64),
		dir:   filepath.Clean(dir),
	}
}
