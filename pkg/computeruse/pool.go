package computeruse

import "sync"

// DesktopFactory creates a fresh Desktop instance.
type DesktopFactory func() (Desktop, error)

// DesktopPool reuses Desktop instances across tasks. It mirrors the
// browseragent browser pool but is simpler: desktops are stateless exec
// sessions, so there is no health check.
type DesktopPool struct {
	mu        sync.Mutex
	available chan Desktop
	factory   DesktopFactory
	maxSize   int
}

// NewDesktopPool creates a pool of up to maxSize desktops created via factory.
func NewDesktopPool(maxSize int, factory DesktopFactory) *DesktopPool {
	if maxSize <= 0 {
		maxSize = 3
	}
	if factory == nil {
		factory = func() (Desktop, error) { return NewLocalDesktop(), nil }
	}
	return &DesktopPool{
		available: make(chan Desktop, maxSize),
		factory:   factory,
		maxSize:   maxSize,
	}
}

// Acquire returns an idle desktop from the pool, or creates a new one.
func (p *DesktopPool) Acquire() (Desktop, error) {
	select {
	case d := <-p.available:
		return d, nil
	default:
		return p.factory()
	}
}

// Release returns a desktop to the pool, closing it if the pool is full.
func (p *DesktopPool) Release(d Desktop) {
	if d == nil {
		return
	}
	select {
	case p.available <- d:
	default:
		_ = d.Close()
	}
}

// Close drains and closes all idle desktops in the pool.
func (p *DesktopPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		select {
		case d := <-p.available:
			_ = d.Close()
		default:
			return
		}
	}
}

// Stats returns pool statistics.
func (p *DesktopPool) Stats() map[string]int {
	return map[string]int{
		"available": len(p.available),
		"max_size":  p.maxSize,
	}
}

// ---- global pool ----

var (
	globalPoolMu  sync.Mutex
	globalPool    *DesktopPool
	globalFactory DesktopFactory = func() (Desktop, error) { return NewLocalDesktop(), nil }
)

// SetDesktopPoolFactory overrides how the global pool creates new desktops.
// Call once at startup to point at an HTTP desktop sidecar instead of local
// xdotool. It closes and resets the existing global pool.
func SetDesktopPoolFactory(factory DesktopFactory) {
	globalPoolMu.Lock()
	defer globalPoolMu.Unlock()
	if globalPool != nil {
		globalPool.Close()
		globalPool = nil
	}
	if factory != nil {
		globalFactory = factory
	}
}

// AcquireDesktop returns a desktop from the global pool, wrapped so Close()
// returns it to the pool instead of destroying it.
func AcquireDesktop() (Desktop, error) {
	globalPoolMu.Lock()
	if globalPool == nil {
		globalPool = NewDesktopPool(3, globalFactory)
	}
	pool := globalPool
	globalPoolMu.Unlock()

	d, err := pool.Acquire()
	if err != nil {
		return nil, err
	}
	return &pooledDesktop{Desktop: d, pool: pool}, nil
}

// pooledDesktop wraps a Desktop so Close() releases it back to its pool.
type pooledDesktop struct {
	Desktop
	pool *DesktopPool
}

// Close returns the desktop to the pool (or closes it if the pool is full).
func (p *pooledDesktop) Close() error {
	p.pool.Release(p.Desktop)
	return nil
}
