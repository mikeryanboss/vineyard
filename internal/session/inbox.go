package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Inbox stores peer messages queued for sessions that are busy when messaged.
//
// Two locks per session coordinate senders and drainers without races:
//   - <id>.lock guards reads and writes of <id>.json and handoffs of
//     <id>.deliver.lock.
//   - <id>.deliver.lock is held by the single process delivering or polling to
//     deliver for <id>. A holder releases it only while holding <id>.lock after
//     seeing an empty queue (FinishIfEmpty), or before spawning a drainer, so
//     an Enqueue that sees <id>.deliver.lock busy is guaranteed its holder will
//     not exit before seeing the new message.
type Inbox struct {
	dir string
}

// NewInbox returns the peer-message inbox for a project data directory.
func NewInbox(projectDir string) Inbox {
	return Inbox{dir: filepath.Join(projectDir, "inbox")}
}

func (in Inbox) queuePath(id string) string   { return filepath.Join(in.dir, id+".json") }
func (in Inbox) lockPath(id string) string    { return filepath.Join(in.dir, id+".lock") }
func (in Inbox) deliverPath(id string) string { return filepath.Join(in.dir, id+".deliver.lock") }

// Delivery is exclusive ownership of delivering queued messages to one session.
type Delivery struct {
	inbox   Inbox
	id      string
	release func()
}

// Enqueue appends message to id's queue. It returns a Delivery when no other
// process is currently delivering to id, and nil when an active holder will
// pick the message up.
func (in Inbox) Enqueue(id, message string) (*Delivery, error) {
	unlock, err := lock(in.lockPath(id), true)
	if err != nil {
		return nil, err
	}
	defer unlock()

	msgs, err := in.load(id)
	if err != nil {
		return nil, err
	}
	if err := in.save(id, append(msgs, message)); err != nil {
		return nil, err
	}
	deliver, err := lock(in.deliverPath(id), false)
	if errors.Is(err, errBusy) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Delivery{inbox: in, id: id, release: deliver}, nil
}

// Claim acquires delivery ownership for id when messages are waiting and no
// other process holds it. It returns nil when the queue is empty or already
// claimed.
func (in Inbox) Claim(id string) (*Delivery, error) {
	unlock, err := lock(in.lockPath(id), true)
	if err != nil {
		return nil, err
	}
	defer unlock()

	msgs, err := in.load(id)
	if err != nil || len(msgs) == 0 {
		return nil, err
	}
	deliver, err := lock(in.deliverPath(id), false)
	if errors.Is(err, errBusy) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Delivery{inbox: in, id: id, release: deliver}, nil
}

// Discard removes any queued messages for id.
func (in Inbox) Discard(id string) error {
	if _, err := os.Stat(in.dir); os.IsNotExist(err) {
		return nil
	}
	unlock, err := lock(in.lockPath(id), true)
	if err != nil {
		return err
	}
	defer unlock()
	err = os.Remove(in.queuePath(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Pop removes and returns all queued messages for the session in FIFO order.
func (d *Delivery) Pop() ([]string, error) {
	unlock, err := lock(d.inbox.lockPath(d.id), true)
	if err != nil {
		return nil, err
	}
	defer unlock()

	msgs, err := d.inbox.load(d.id)
	if err != nil || len(msgs) == 0 {
		return nil, err
	}
	if err := os.Remove(d.inbox.queuePath(d.id)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return msgs, nil
}

// FinishIfEmpty releases delivery ownership if and only if no messages are
// waiting. When false is returned, ownership is still held and new messages
// have arrived.
func (d *Delivery) FinishIfEmpty() (bool, error) {
	unlock, err := lock(d.inbox.lockPath(d.id), true)
	if err != nil {
		return false, err
	}
	defer unlock()

	msgs, err := d.inbox.load(d.id)
	if err != nil {
		return false, err
	}
	if len(msgs) > 0 {
		return false, nil
	}
	d.Release()
	return true, nil
}

// Release drops delivery ownership without checking the queue, as before
// handing off to a background drainer or stopping on error.
func (d *Delivery) Release() {
	if d.release != nil {
		d.release()
		d.release = nil
	}
}

func (in Inbox) load(id string) ([]string, error) {
	path := in.queuePath(id)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var msgs []string
	if err := json.Unmarshal(content, &msgs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return msgs, nil
}

func (in Inbox) save(id string, msgs []string) error {
	if err := os.MkdirAll(in.dir, 0o755); err != nil {
		return err
	}
	content, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(in.dir, ".inbox-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(content, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), in.queuePath(id))
}
