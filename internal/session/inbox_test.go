package session

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

func TestInbox_HandoffAndStoreRemove(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Add(Session{ID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "inbox")); !os.IsNotExist(err) {
		t.Fatal("Remove created inbox directory when no message was ever queued")
	}

	inbox := NewInbox(dir)
	d1, err := inbox.Enqueue("a", "first")
	if err != nil || d1 == nil {
		t.Fatalf("first Enqueue = %v, %v; want Delivery", d1, err)
	}
	// A second sender arriving while d1 holds delivery gets nil, relying on d1.
	d2, err := inbox.Enqueue("a", "second")
	if err != nil || d2 != nil {
		t.Fatalf("second Enqueue while held = %v, %v; want nil, nil", d2, err)
	}

	msgs, err := d1.Pop()
	if err != nil || !slices.Equal(msgs, []string{"first", "second"}) {
		t.Fatalf("Pop = %v, %v; want [first second]", msgs, err)
	}

	// A message arriving after Pop but before FinishIfEmpty keeps d1 active.
	if d3, err := inbox.Enqueue("a", "third"); err != nil || d3 != nil {
		t.Fatalf("third Enqueue before FinishIfEmpty = %v, %v; want nil, nil", d3, err)
	}
	empty, err := d1.FinishIfEmpty()
	if err != nil || empty {
		t.Fatalf("FinishIfEmpty with waiting message = %v, %v; want false, nil", empty, err)
	}

	msgs, err = d1.Pop()
	if err != nil || !slices.Equal(msgs, []string{"third"}) {
		t.Fatalf("second Pop = %v, %v; want [third]", msgs, err)
	}
	empty, err = d1.FinishIfEmpty()
	if err != nil || !empty {
		t.Fatalf("FinishIfEmpty when drained = %v, %v; want true, nil", empty, err)
	}

	// Once released, a fresh Enqueue claims delivery again, and Store.Remove clears it.
	if err := store.Add(Session{ID: "b"}); err != nil {
		t.Fatal(err)
	}
	d4, err := inbox.Enqueue("b", "drop me")
	if err != nil || d4 == nil {
		t.Fatalf("Enqueue after release = %v, %v; want Delivery", d4, err)
	}
	d4.Release()
	if err := store.Remove("b"); err != nil {
		t.Fatal(err)
	}
	if claimed, err := inbox.Claim("b"); err != nil || claimed != nil {
		t.Fatalf("Claim after Store.Remove = %v, %v; want nil, nil", claimed, err)
	}
}

// Concurrent senders racing with a consumer's FinishIfEmpty must never strand
// a message without a Delivery owner or deliver one twice.
func TestInbox_ConcurrentHandoffNeverStrandsMessages(t *testing.T) {
	inbox := NewInbox(t.TempDir())
	const senders, perSender = 4, 25

	var mu sync.Mutex
	var delivered []string
	drain := func(d *Delivery) {
		for {
			msgs, err := d.Pop()
			if err != nil {
				t.Error(err)
				d.Release()
				return
			}
			mu.Lock()
			delivered = append(delivered, msgs...)
			mu.Unlock()
			empty, err := d.FinishIfEmpty()
			if err != nil {
				t.Error(err)
				d.Release()
				return
			}
			if empty {
				return
			}
		}
	}

	var wg sync.WaitGroup
	for s := range senders {
		wg.Go(func() {
			for i := range perSender {
				d, err := inbox.Enqueue("target", fmt.Sprintf("s%d-%d", s, i))
				if err != nil {
					t.Error(err)
					return
				}
				if d != nil {
					drain(d)
				}
			}
		})
	}
	wg.Wait()

	if len(delivered) != senders*perSender {
		t.Fatalf("delivered %d messages, want %d", len(delivered), senders*perSender)
	}
	slices.Sort(delivered)
	if len(slices.Compact(delivered)) != senders*perSender {
		t.Fatal("duplicate message delivered")
	}
}
