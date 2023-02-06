package tail

func (fx *Fx) Poll(tx *Tx) {
	tx.after = fx.watcher.after
	go tx.poll(fx.watcher.interval, fx.watcher.timeout)
}

func (fx *Fx) Inotify(tx *Tx) {
	tx.after = fx.watcher.after
	go tx.inotify(fx.watcher.interval)
}

func (fx *Fx) doWatcher(tx *Tx) {

	switch fx.watcher.mode {
	case WTPoll:
		fx.Poll(tx)
	case WTInotify:
		fx.Inotify(tx)
	}

	return
}
