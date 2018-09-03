package p2p

import (
	"io"
	"sync"

	net "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

const CMGR_TAG = "stream-fwd"

// Stream holds information on active incoming and outgoing p2p streams.
type Stream struct {
	id uint64

	Protocol protocol.ID

	OriginAddr ma.Multiaddr
	TargetAddr ma.Multiaddr

	Local  manet.Conn
	Remote net.Stream

	Registry *StreamRegistry

	cleanup func()
}

// Close closes stream endpoints and deregisters it
func (s *Stream) Close() error {
	s.Local.Close()
	s.Remote.Close()
	s.cleanup()
	s.Registry.Deregister(s.id)
	return nil
}

// Reset closes stream endpoints and deregisters it
func (s *Stream) Reset() error {
	s.Local.Close()
	s.Remote.Reset()
	s.Registry.Deregister(s.id)
	return nil
}

func (s *Stream) startStreaming() {
	go func() {
		_, err := io.Copy(s.Local, s.Remote)
		if err != nil {
			s.Reset()
		} else {
			s.Close()
		}
	}()

	go func() {
		_, err := io.Copy(s.Remote, s.Local)
		if err != nil {
			s.Reset()
		} else {
			s.Close()
		}
	}()
}

// StreamRegistry is a collection of active incoming and outgoing proto app streams.
type StreamRegistry struct {
	sync.Mutex

	Streams map[uint64]*Stream
	nextID  uint64
}

// Register registers a stream to the registry
func (r *StreamRegistry) Register(streamInfo *Stream) {
	r.Lock()
	defer r.Unlock()

	streamInfo.id = r.nextID
	r.Streams[r.nextID] = streamInfo
	r.nextID++
}

// Deregister deregisters stream from the registry
func (r *StreamRegistry) Deregister(streamID uint64) {
	r.Lock()
	defer r.Unlock()

	delete(r.Streams, streamID)
}
