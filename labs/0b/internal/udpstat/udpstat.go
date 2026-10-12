// Package udpstat считает потери, дубликаты и переупорядочивание UDP-ответов.
package udpstat

import "time"

// Reply — один принятый ответ: номер датаграммы и измеренный RTT.
type Reply struct {
	Seq uint32
	RTT time.Duration
}

// Counts — счётчики итогов; сравниваются целиком, поэтому отделены от среза RTTs.
type Counts struct{ Sent, Received, Lost, Duplicates, Reordered int }

// Result — итог серии: счётчики и RTT уникальных ответов (дубликаты не учитываются).
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
	var seenAny bool
	for _, rep := range replies {
		if seen[rep.Seq] {
			r.Duplicates++
			continue
		}
		seen[rep.Seq] = true
		r.Received++
		r.RTTs = append(r.RTTs, rep.RTT)
		if seenAny && rep.Seq < maxSeq {
			r.Reordered++
		}
		if !seenAny || rep.Seq > maxSeq {
			maxSeq = rep.Seq
		}
		seenAny = true
	}
	r.Lost = sent - r.Received
	return r
}
