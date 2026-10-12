// Package udpstat считает потери, дубликаты и переупорядочивание UDP-ответов.
package udpstat

import "time"

type Reply struct {
	Seq uint32
	RTT time.Duration
}

type Counts struct{ Sent, Received, Lost, Duplicates, Reordered int }

type Result struct {
	Counts
	RTTs []time.Duration
}

// Analyze: replies — в порядке прихода. Переупорядоченным считается ответ,
// пришедший после ответа с большим номером.
func Analyze(sent int, replies []Reply) Result {
	seen := make(map[uint32]bool, len(replies))
	r := Result{Counts: Counts{Sent: sent}}
	var maxSeq uint32
	var any bool
	for _, rep := range replies {
		if seen[rep.Seq] {
			r.Duplicates++
			continue
		}
		seen[rep.Seq] = true
		r.Received++
		r.RTTs = append(r.RTTs, rep.RTT)
		if any && rep.Seq < maxSeq {
			r.Reordered++
		}
		if !any || rep.Seq > maxSeq {
			maxSeq = rep.Seq
		}
		any = true
	}
	r.Lost = sent - r.Received
	return r
}
