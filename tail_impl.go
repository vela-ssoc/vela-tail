package tail

import (
	"github.com/vela-ssoc/vela-kit/audit"
	"sync/atomic"
)

func (t *tail) newFx(path string) *Fx {
	return newFx(t.tom, path, t.handle)
}

func (t *tail) add(raw line) *tail {
	add := raw.add
	if add == nil {
		add = t.cfg.add
	}

	if add == nil {
		return t
	}

	raw.value = add(raw.value)
	return t
}

func (t *tail) enc(raw line) *tail {
	enc := raw.enc
	if enc == nil {
		enc = t.cfg.enc
	}

	if enc == nil {
		return t
	}

	raw.value = enc(raw.value)
	return t
}

func (t *tail) push(chunk []byte) {
	if t.cfg.sdk == nil {
		return
	}

	wn, err := t.cfg.sdk.Write(chunk)
	if err != nil {
		xEnv.Errorf("%s output write error %v", t.Name(), err)
		return
	}
	atomic.AddUint64(&t.wn, uint64(wn))
}

func (t *tail) toPipe(raw line) {
	//调用接口
	t.cfg.pipe.Do(raw.value, t.cfg.co, func(err error) {
		audit.Errorf("%s pipe call fail %v", t.Name(), err).From(t.CodeVM()).Put()
	})
}

func (t *tail) handle(raw line, e error) {

	if e != nil {
		return
	}

	//pipe
	t.toPipe(raw)

	//限速
	t.limit.wait()

	//发送数据
	t.queue <- raw
}

func (t *tail) output(idx int) {
	for raw := range t.queue {
		raw.Enc(t.cfg.enc)
		raw.Add(t.cfg.add)
		t.push(raw.byte())
	}
	audit.Errorf("%s %d output thread exit", t.Name(), idx).Log().From(t.CodeVM()).Put()
}
