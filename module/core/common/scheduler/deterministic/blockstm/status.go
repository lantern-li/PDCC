package blockstm

import "sync"

type Status uint

const (
	StatusReadyToExecute Status = iota
	StatusExecuting
	StatusExecuted
	StatusAborting
	StatusSuspended
)

// StatusEntry 每笔交易的执行状态
type StatusEntry struct {
	sync.Mutex

	incarnation Incarnation
	status      Status

	//cond *Condvar todo：遇到未完成写入时等待”的优化路径 暂时先不优化这个
}

func (s *StatusEntry) TrySetExecuting() (Incarnation, bool) {
	s.Lock()

	if s.status == StatusReadyToExecute { // todo：incarnation在哪儿自增的
		s.status = StatusExecuting
		incarnation := s.incarnation

		s.Unlock()
		return incarnation, true
	}

	s.Unlock()
	return 0, false
}

func (s *StatusEntry) IsExecuted() (ok bool, incarnation Incarnation) {
	s.Lock()

	if s.status == StatusExecuted {
		ok = true
		incarnation = s.incarnation
	}

	s.Unlock()
	return
}
