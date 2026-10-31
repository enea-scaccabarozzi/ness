package notify

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

var (
	// ErrNoChannel is returned for a channel name not in [notify.channels] (R-NTF-002).
	ErrNoChannel = errors.New("no such channel")
	// ErrEmpty is returned for a note with no blocks: nothing to send.
	ErrEmpty = errors.New("the note is empty")

	issueNumber = regexp.MustCompile(`^\d+$`)
)

// PartialError is a post that stopped at a part: the parts before it were sent and recorded,
// so the same post resumes at Sent+1 (R-NTF-007).
type PartialError struct {
	Sent, Parts int
	Err         error
}

func (e *PartialError) Error() string {
	sent := "nothing was sent"
	switch {
	case e.Sent == 1:
		sent = "part 1 was sent"
	case e.Sent > 1:
		sent = fmt.Sprintf("parts 1-%d were sent", e.Sent)
	}
	return fmt.Sprintf("part %d of %d failed: %v; %s; run the same post again to resume at part %d",
		e.Sent+1, e.Parts, e.Err, sent, e.Sent+1)
}

func (e *PartialError) Unwrap() error { return e.Err }

// Poster posts one note: the channel by name (built by the caller, credentials included), the
// store for threads and progress, the clock.
//
// Delivery is at-least-once: a crash between a channel's Post and the MarkSent that follows
// re-sends that one part on the retry.
type Poster struct {
	Channel func(name string) (Channel, error)
	Store   *store.Store
	Now     func() time.Time
}

// Request is one ness post: the note, the verbatim text that replaces its rendering when set
// (R-NTF-004), the thread key, and the writer a dry run prints payloads to (nil: a real post).
type Request struct {
	Channel, Markdown, Verbatim, Thread string
	DryRun                              io.Writer
}

// PostKey identifies a post for resumption: channel, thread key and every rendered part.
func PostKey(channel, thread string, parts []Part) string {
	h := sha256.New()
	for _, s := range append([]string{channel, thread}, texts(parts)...) {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func texts(parts []Part) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, p.Text())
	}
	return out
}

// Post renders, splits, prints (dry run) or sends each part in order, recording the thread
// after the first and the progress after every part (R-NTF-003, R-NTF-005, R-NTF-007). It
// returns how many parts the note has.
func (p Poster) Post(r Request) (int, error) {
	ch, err := p.Channel(r.Channel)
	if err != nil {
		return 0, err
	}
	parts, err := p.parts(ch, r)
	if err != nil {
		return 0, err
	}
	if cerr := checkThread(ch, r); cerr != nil {
		return 0, cerr
	}
	thread, err := p.thread(ch, r)
	if err != nil {
		return 0, err
	}
	if r.DryRun != nil {
		return len(parts), printPayloads(r.DryRun, ch, parts, thread)
	}
	return len(parts), p.send(ch, r, parts, thread)
}

// checkThread refuses a channel that needs a thread and has none, and a GitHub thread that is
// not an issue or PR number (the key is the number there).
func checkThread(ch Channel, r Request) error {
	if ch.NeedsThread() && r.Thread == "" {
		return fmt.Errorf("channel %s posts into a thread; pass --thread KEY", r.Channel)
	}
	if ch.Kind() == "github" && r.Thread != "" && !issueNumber.MatchString(r.Thread) {
		return fmt.Errorf("channel %s: the thread must be an issue or PR number, not %q", r.Channel, r.Thread)
	}
	return nil
}

func printPayloads(w io.Writer, ch Channel, parts []Part, thread string) error {
	for _, part := range parts {
		if _, err := fmt.Fprintf(w, "%s\n", ch.Payload(part, thread)); err != nil {
			return err
		}
	}
	return nil
}

// thread is the recorded platform thread of the key, "" for none; a channel without platform
// thread ids takes the key itself.
func (p Poster) thread(ch Channel, r Request) (string, error) {
	if r.Thread == "" {
		return "", nil
	}
	if ch.ThreadIsKey() {
		return r.Thread, nil
	}
	return p.Store.Thread(r.Channel, r.Thread)
}

// parts is the verbatim text as one part, or the rendered and split note.
func (p Poster) parts(ch Channel, r Request) ([]Part, error) {
	if r.Verbatim != "" {
		return []Part{{Blocks: []string{r.Verbatim}}}, nil
	}
	blocks := Parse(r.Markdown)
	if len(blocks) == 0 {
		return nil, ErrEmpty
	}
	return Split(ch, blocks)
}

func (p Poster) send(ch Channel, r Request, parts []Part, thread string) (err error) {
	key := PostKey(r.Channel, r.Thread, parts)
	owner, err := newOwner()
	if err != nil {
		return err
	}
	sent, err := p.Store.BeginPost(key, r.Channel, len(parts), p.Now(), owner)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.Store.ReleasePost(key, owner)) }()
	for i := sent; i < len(parts); i++ {
		id, perr := ch.Post(parts[i], thread)
		if perr != nil {
			return &PartialError{Sent: i, Parts: len(parts), Err: perr}
		}
		var th store.ThreadRef
		if thread == "" && r.Thread != "" {
			thread, th = id, store.ThreadRef{Channel: r.Channel, Key: r.Thread, ID: id}
		}
		if merr := p.Store.MarkSent(key, i+1, p.Now(), th); merr != nil {
			return merr
		}
	}
	return nil
}

// newOwner identifies this run's claim: the pid and a random token.
func newOwner() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%s", os.Getpid(), hex.EncodeToString(b)), nil
}
