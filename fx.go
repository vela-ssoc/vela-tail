package tail

import (
	"bufio"
	cond "github.com/vela-ssoc/vela-cond"
	"github.com/vela-ssoc/vela-kit/audit"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
	"github.com/vela-ssoc/vela-kit/pipe"
	"gopkg.in/tomb.v2"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	WTPoll modeT = iota + 1
	WTInotify
)

type modeT uint

type Watcher struct {
	wait     time.Duration
	watcher  bool
	mode     modeT
	after    time.Duration
	interval time.Duration
	timeout  time.Duration
	onEOF    *pipe.Chains
}

type Fx struct {
	path  string
	bkt   []string
	delim byte
	err   error
	fd    *os.File
	rd    *bufio.Reader

	buffer int
	seek   int64
	stat   os.FileInfo
	stime  time.Time

	enc    func([]byte) []byte
	add    func([]byte) []byte
	handle func(line, error)

	watcher *Watcher
	co      *lua.LState
	pipe    *pipe.Chains
	cnd     *cond.Cond
	tom     *tomb.Tomb
	tag     map[string]lua.LValue
}

func (fx *Fx) openFile() error {
	fd, err := openFile(fx.path)
	if err != nil {
		fx.err = err
		return err
	}

	fx.fd = fd
	fx.rd = bufio.NewReaderSize(fd, fx.buffer)
	fx.offset()
	fx.err = nil
	fx.stime = time.Now()

	xEnv.Spawn(0, fx.readline)
	audit.Debug("%s fx open succeed record [%d]", fx.path, fx.seek).From(fx.co.CodeVM()).Put()
	return nil
}

func (fx *Fx) open() error {

	//是否开启等待
	if fx.watcher.wait < 0 {
		return fx.openFile()
	}

	er := fx.openFile()
	if er == nil {
		return nil
	}

	tk := time.NewTicker(fx.watcher.wait)
	defer tk.Stop()

	for {

		select {

		case <-tk.C:
			er = fx.openFile()
			if er == nil {
				return nil
			}

			audit.Errorf("%s open file fail %v", fx.path, er).From(fx.co.CodeVM()).Put()

		case <-fx.tom.Dying():
			audit.Errorf("%s wait exit", fx.path).From(fx.co.CodeVM()).Put()
			return nil

		}
	}

	return nil
}

func (fx *Fx) save() {
	seek, e := fx.fd.Seek(0, io.SeekCurrent)
	if e != nil {
		xEnv.Infof("%s current seek error %v", fx.path, e)
		return
	}

	if fx.fd == nil {
		xEnv.Errorf("current %s file is nil", fx.path)
		return
	}

	bkt := xEnv.Bucket(fx.bkt...)
	if bkt == nil {
		xEnv.Errorf("%s fx current bucket empty", fx.path)
		return
	}

	err := bkt.Store(fx.path, seek, 0)
	if err != nil {
		xEnv.Errorf("save %s seek record error %v", fx.path, err)
		return
	}

	xEnv.Infof("%s save seek record [%d]", fx.path, seek)
}

func (fx *Fx) offset() {

	if fx.fd == nil {
		xEnv.Infof("tail %s fd not found", fx.path)
		return
	}

	bkt := xEnv.Bucket(fx.bkt...)
	if bkt == nil {
		xEnv.Errorf("%s fx current bucket empty", fx.path)
		return
	}

	seek := bkt.Int64(fx.path)
	stat, _ := fx.fd.Stat()
	size := stat.Size()
	if seek > size {
		xEnv.Infof("tail offset record [%d] > [%d]", seek, size)
		fx.seek = 0
	} else {
		fx.seek = seek
	}

	fx.fd.Seek(fx.seek, 0)
	xEnv.Infof("%s tail position of %d", fx.path, fx.seek)
	return

}

func (fx *Fx) onEOF(raw []byte) {
	if fx.watcher.onEOF.Len() == 0 {
		fx.doWatcher(newTx(fx, raw))
		return
	}

	fx.watcher.onEOF.Do(newTx(fx, raw), fx.co, func(err error) {
		audit.Errorf("tail %s fx on eof pipe call fail %v", fx.path, err).From(fx.co.CodeVM()).High().Put()
	})
}

func (fx *Fx) onRead(raw []byte) {
	if len(raw) == 0 {
		return
	}

	fx.pipe.Do(raw, fx.co, func(err error) {
		audit.Errorf("tail %s fx on read pipe call fail %v", fx.path, err).From(fx.co.CodeVM()).High().Put()
	})
}

func (fx *Fx) filter(raw []byte) bool {
	if fx.cnd == nil {
		return true
	}

	return fx.cnd.Match(auxlib.B2S(raw))
}

func (fx *Fx) Handle(raw []byte) {

	rn := len(raw)
	if rn <= 1 {
		return
	}

	if raw[rn-1] == fx.delim {
		raw = raw[:rn-1]
	}

	if !fx.filter(raw) {
		return
	}

	fx.onRead(raw)
	fx.handle(line{raw, fx.enc, fx.add}, nil)
}

func (fx *Fx) close() {
	_ = fx.fd.Close()
	fx.cnd = nil
}

func (fx *Fx) exit() {
	fx.watcher.onEOF = nil
	fx.cnd = nil
	xEnv.Errorf("%s tx exit when eof", fx.path)
}

func (fx *Fx) readline() {
	defer fx.close()

	for {
		select {

		case <-fx.tom.Dying():
			fx.save()
			audit.Errorf("tail %s readline exit", fx.path).From(fx.co.CodeVM()).High().Put()
			return

		default:
			raw, err := fx.rd.ReadBytes(fx.delim)
			if err == nil {
				fx.Handle(raw)
				continue
			}

			if err.Error() == "EOF" {
				fx.Handle(raw)
				fx.save()
				fx.onEOF(raw)
				xEnv.Infof("%s file eof", fx.path)
				//轮询监控
				return
			}
			audit.Errorf("tail %s fx read line raw error %v", fx.path, err).
				From(fx.co.CodeVM()).High().Put()
			//todo
		}
	}
}

func newFx(tom *tomb.Tomb, value string, handle func(line, error)) *Fx {
	path, e := filepath.Abs(filepath.Clean(value))
	if e != nil {
		path = filepath.Clean(value)
	}

	fx := &Fx{
		delim:  '\n',
		buffer: 4096,
		tom:    tom,
		path:   path,
		handle: handle,
		pipe:   pipe.New(),
	}

	fx.watcher = &Watcher{
		wait:     10 * time.Second,
		mode:     WTPoll,
		interval: 5 * time.Second,
		timeout:  20 * 365 * 24 * time.Hour,
		onEOF:    pipe.New(),
	}

	return fx
}
