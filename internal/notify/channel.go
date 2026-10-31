package notify

// Channel is the channel seam (design §2): render Markdown, post, reply in a thread by key.
// Payload is the exact body Post sends, for --dry-run (R-NTF-003); Post sends one part in the
// thread (an empty thread starts one) and returns the id a reply addresses. NeedsThread is
// true for a channel that cannot post without one (GitHub: a comment needs its issue).
// ThreadIsKey is true for a channel with no platform thread ids (GitHub, command): the thread
// key itself is passed as the thread and nothing is recorded for it.
type Channel interface {
	Renderer
	Kind() string
	NeedsThread() bool
	ThreadIsKey() bool
	Payload(p Part, thread string) []byte
	Post(p Part, thread string) (id string, err error)
}
