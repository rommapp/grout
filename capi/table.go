package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"

	"github.com/rommapp/grout/api"
)

type sink func(callID int64, payload []byte)

type backend interface {
	Call(ctx context.Context, method string, args []byte, emit func(api.Event)) []byte
	Close() error
}

type call struct {
	cancel     context.CancelFunc
	silenced   bool
	delivering map[uint64]int
}

func (c *call) silence() {
	c.silenced = true
	c.cancel()
}

func (c *call) threadsExcept(tid uint64) []uint64 {
	var out []uint64
	for th := range c.delivering {
		if th != tid {
			out = append(out, th)
		}
	}
	return out
}

type instance struct {
	handle  uintptr
	b       backend
	sink    sink
	closing bool
	closed  bool
	calls   map[int64]*call
}

type blockers struct {
	threads []uint64
	other   bool
}

func (inst *instance) closeBlockers(tid uint64) blockers {
	var b blockers
	for _, c := range inst.calls {
		if len(c.delivering) == 0 {
			b.other = true
			continue
		}
		b.threads = append(b.threads, c.threadsExcept(tid)...)
	}
	return b
}

func (inst *instance) deliveryBlockers(tid uint64) blockers {
	var b blockers
	for _, c := range inst.calls {
		b.threads = append(b.threads, c.threadsExcept(tid)...)
	}
	return b
}

type table struct {
	mu        sync.Mutex
	cond      *sync.Cond
	threadID  func() uint64
	next      uintptr
	instances map[uintptr]*instance
	waiters   map[uint64]func() blockers
}

func newTable(threadID func() uint64) *table {
	t := &table{
		threadID:  threadID,
		instances: map[uintptr]*instance{},
		waiters:   map[uint64]func() blockers{},
	}
	t.cond = sync.NewCond(&t.mu)
	return t
}

func failure(code api.Code, format string, args ...any) []byte {
	raw, err := json.Marshal(api.Envelope{Error: &api.ErrorBody{Code: code, Message: fmt.Sprintf(format, args...)}})
	if err != nil {
		return []byte(`{"ok":false,"error":{"code":"internal","message":"encoding error"}}`)
	}
	return raw
}

type wireEvent struct {
	CallID int64 `json:"call_id"`
	api.Event
}

func eventJSON(callID int64, e api.Event) []byte {
	raw, err := json.Marshal(wireEvent{CallID: callID, Event: e})
	if err != nil {
		return []byte(fmt.Sprintf(`{"call_id":%d}`, callID))
	}
	return raw
}

func (t *table) open(options []byte) (uintptr, []byte) {
	if len(options) == 0 {
		return 0, failure(api.CodeBadRequest, "options are required")
	}
	o, err := api.DecodeOpenOptions(options)
	if err != nil {
		return 0, api.ErrorEnvelope(err)
	}
	srv, err := api.Open(context.Background(), o)
	if err != nil {
		return 0, api.ErrorEnvelope(err)
	}
	return t.add(srv), nil
}

func (t *table) add(b backend) uintptr {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	h := t.next
	t.instances[h] = &instance{handle: h, b: b, calls: map[int64]*call{}}
	return h
}

func (t *table) call(h uintptr, id int64, method string, args []byte) []byte {
	t.mu.Lock()
	inst := t.instances[h]
	switch {
	case inst == nil:
		t.mu.Unlock()
		return failure(api.CodeBadRequest, "unknown handle %d", h)
	case inst.closing:
		t.mu.Unlock()
		return failure(api.CodeBadRequest, "handle %d is closed", h)
	}
	if _, dup := inst.calls[id]; dup {
		t.mu.Unlock()
		return failure(api.CodeBadRequest, "call id %d is already in flight", id)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &call{cancel: cancel, delivering: map[uint64]int{}}
	inst.calls[id] = c
	t.mu.Unlock()
	defer func() {
		cancel()
		t.mu.Lock()
		delete(inst.calls, id)
		t.forget(inst)
		t.cond.Broadcast()
		t.mu.Unlock()
	}()
	return inst.b.Call(ctx, method, args, func(e api.Event) { t.deliver(inst, c, id, e) })
}

func (t *table) deliver(inst *instance, c *call, id int64, e api.Event) {
	payload := eventJSON(id, e)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	tid := t.threadID()
	t.mu.Lock()
	s := inst.sink
	if s == nil || inst.closing || c.silenced {
		t.mu.Unlock()
		return
	}
	c.delivering[tid]++
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		c.delivering[tid]--
		if c.delivering[tid] == 0 {
			delete(c.delivering, tid)
		}
		t.cond.Broadcast()
		t.mu.Unlock()
	}()
	s(id, payload)
}

func (t *table) setSink(h uintptr, s sink) {
	tid := t.threadID()
	t.mu.Lock()
	defer t.mu.Unlock()
	inst := t.instances[h]
	if inst == nil {
		return
	}
	inst.sink = s
	t.wait(tid, func() blockers { return inst.deliveryBlockers(tid) })
}

func (t *table) cancel(h uintptr, id int64) {
	tid := t.threadID()
	t.mu.Lock()
	defer t.mu.Unlock()
	inst := t.instances[h]
	if inst == nil {
		return
	}
	c := inst.calls[id]
	if c == nil {
		return
	}
	c.silence()
	t.wait(tid, func() blockers { return blockers{threads: c.threadsExcept(tid)} })
}

func (t *table) close(h uintptr) {
	tid := t.threadID()
	t.mu.Lock()
	inst := t.instances[h]
	if inst == nil {
		t.mu.Unlock()
		return
	}
	if !inst.closing {
		inst.closing = true
		for _, c := range inst.calls {
			c.silence()
		}
	}
	t.wait(tid, func() blockers { return inst.closeBlockers(tid) })
	t.mu.Unlock()
	inst.b.Close()
	t.mu.Lock()
	inst.closed = true
	t.forget(inst)
	t.mu.Unlock()
}

func (t *table) forget(inst *instance) {
	if inst.closed && len(inst.calls) == 0 && t.instances[inst.handle] == inst {
		delete(t.instances, inst.handle)
	}
}

func (t *table) wait(tid uint64, waiting func() blockers) {
	prev, nested := t.waiters[tid]
	t.waiters[tid] = waiting
	t.cond.Broadcast()
	for t.blocked(tid, waiting()) {
		t.cond.Wait()
	}
	if nested {
		t.waiters[tid] = prev
	} else {
		delete(t.waiters, tid)
	}
}

func (t *table) blocked(tid uint64, b blockers) bool {
	if b.other {
		return true
	}
	for _, th := range b.threads {
		if !t.reaches(th, tid, map[uint64]bool{}) {
			return true
		}
	}
	return false
}

func (t *table) reaches(from, target uint64, seen map[uint64]bool) bool {
	waiting := t.waiters[from]
	if waiting == nil {
		return false
	}
	seen[from] = true
	for _, th := range waiting().threads {
		if th == target {
			return true
		}
		if !seen[th] && t.reaches(th, target, seen) {
			return true
		}
	}
	return false
}
