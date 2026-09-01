// Package discord provides a rate-limited webhook notifier used by the
// logger to forward warn/error log entries to a Discord channel.
//
// The notifier is intentionally side-effect-free on failure (no logging,
// no panics) because it sits behind the global zap logger — recursive
// errors would amplify the problem it's trying to report.
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Event is a single log entry that may be forwarded to Discord.
type Event struct {
	Level   string
	Message string
	Caller  string
	Time    time.Time
	Fields  map[string]any
}

// Notifier batches Events and posts them to a Discord webhook URL with
// rate limiting so a burst of identical errors collapses to one summary
// message per debounce window.
//
// Behavior:
//   - The first Notify call after a quiet period sends immediately.
//   - Subsequent calls within MinInterval are appended to a pending batch
//     and flushed when the window elapses.
//   - The notifier never blocks the caller: HTTP work runs on a goroutine.
type Notifier struct {
	webhookURL  string
	httpClient  *http.Client
	minInterval time.Duration
	source      string // optional label included in messages, e.g. service name

	mu       sync.Mutex
	pending  []Event
	lastSent time.Time
	timer    *time.Timer
}

// NewNotifier returns a notifier posting to webhookURL. The minInterval is
// the minimum gap between webhook posts (default 10s if <= 0). The source
// label is prefixed onto Discord messages to identify the service.
func NewNotifier(webhookURL string, minInterval time.Duration, source string) *Notifier {
	if minInterval <= 0 {
		minInterval = 10 * time.Second
	}
	return &Notifier{
		webhookURL:  webhookURL,
		minInterval: minInterval,
		source:      source,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify queues e for sending. Cheap to call from any goroutine; the
// actual HTTP post happens off the calling goroutine.
func (n *Notifier) Notify(e Event) {
	n.mu.Lock()

	n.pending = append(n.pending, e)
	elapsed := time.Since(n.lastSent)

	if elapsed >= n.minInterval {
		toSend := n.pending
		n.pending = nil
		n.lastSent = time.Now()
		if n.timer != nil {
			n.timer.Stop()
			n.timer = nil
		}
		n.mu.Unlock()
		go n.send(toSend)
		return
	}

	if n.timer == nil {
		wait := n.minInterval - elapsed
		n.timer = time.AfterFunc(wait, n.flushPending)
	}
	n.mu.Unlock()
}

func (n *Notifier) flushPending() {
	n.mu.Lock()
	if len(n.pending) == 0 {
		n.timer = nil
		n.mu.Unlock()
		return
	}
	toSend := n.pending
	n.pending = nil
	n.lastSent = time.Now()
	n.timer = nil
	n.mu.Unlock()
	n.send(toSend)
}

func (n *Notifier) send(events []Event) {
	if len(events) == 0 {
		return
	}
	content := n.formatContent(events)
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// formatContent builds the Discord message. Single events get full detail;
// batches show count + first/last samples.
func (n *Notifier) formatContent(events []Event) string {
	const maxBody = 1800 // Discord 2000 char hard limit; leave headroom

	src := ""
	if n.source != "" {
		src = " [`" + n.source + "`]"
	}

	if len(events) == 1 {
		e := events[0]
		emoji := emojiForLevel(e.Level)
		body := fmt.Sprintf("%s **%s**%s `%s`\n`%s` — %s\n%s",
			emoji,
			strings.ToUpper(e.Level),
			src,
			e.Time.UTC().Format("2006-01-02T15:04:05Z"),
			e.Caller,
			escapeMarkdown(e.Message),
			formatFields(e.Fields),
		)
		return truncate(body, maxBody)
	}

	first := events[0]
	last := events[len(events)-1]
	emoji := emojiForLevel(first.Level)

	header := fmt.Sprintf("%s **%d log events**%s from `%s` → `%s`",
		emoji,
		len(events),
		src,
		first.Time.UTC().Format("15:04:05Z"),
		last.Time.UTC().Format("15:04:05Z"),
	)

	// Group by message to show the dominant errors
	counts := make(map[string]int)
	for _, e := range events {
		counts[e.Message]++
	}
	type kv struct {
		Msg   string
		Count int
	}
	items := make([]kv, 0, len(counts))
	for m, c := range counts {
		items = append(items, kv{m, c})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Count > items[j].Count })

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for i, it := range items {
		if i >= 5 {
			b.WriteString(fmt.Sprintf("…and %d more distinct messages\n", len(items)-5))
			break
		}
		fmt.Fprintf(&b, "• ×%d `%s`\n", it.Count, escapeMarkdown(it.Msg))
	}
	fmt.Fprintf(&b, "\nFirst: %s", formatFields(first.Fields))
	if last.Message != first.Message {
		fmt.Fprintf(&b, "\nLast: `%s` %s", escapeMarkdown(last.Message), formatFields(last.Fields))
	}
	return truncate(b.String(), maxBody)
}

func formatFields(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i >= 8 {
			b.WriteString(" …")
			break
		}
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "`%s=%v`", k, truncateField(fields[k]))
	}
	return b.String()
}

func truncateField(v any) string {
	s := fmt.Sprintf("%v", v)
	const max = 80
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// escapeMarkdown neutralizes backticks so the inline-code formatting in
// the message body doesn't break when error text itself contains them.
func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "`", "'")
}

func emojiForLevel(level string) string {
	switch strings.ToLower(level) {
	case "fatal", "panic", "dpanic":
		return ":rotating_light:"
	case "error":
		return ":red_circle:"
	case "warn":
		return ":warning:"
	default:
		return ":information_source:"
	}
}
