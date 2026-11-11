package main

import "time"

type ReceiveMeter struct {
	At      time.Time
	Total   uint64
	Running bool
	Rate    float64
	Samples []float64
}

func (m *ReceiveMeter) Update(now time.Time, total uint64, running bool) {
	if !running {
		m.Running = false
		m.Rate = 0
		m.At = now
		m.Total = total
		return
	}
	if !m.Running || total < m.Total {
		m.Running = true
		m.At = now
		m.Total = total
		m.Rate = 0
		m.Samples = nil
		return
	}
	elapsed := now.Sub(m.At).Seconds()
	if elapsed < 0.5 {
		return
	}
	m.Rate = float64(total-m.Total) / elapsed
	m.Total = total
	m.At = now
	m.Samples = append(m.Samples, m.Rate)
	if len(m.Samples) > 40 {
		m.Samples = m.Samples[len(m.Samples)-40:]
	}
}
