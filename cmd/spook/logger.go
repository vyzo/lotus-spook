package main

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/record"
)

type Logger struct {
	sync.Mutex

	peers map[peer.ID]uint64
	out   *json.Encoder
}

func NewLogger(w io.Writer) *Logger {
	return &Logger{
		peers: make(map[peer.ID]uint64),
		out:   json.NewEncoder(w),
	}
}

func (l *Logger) AddPeer(p peer.ID, peerRecord []byte) {
	l.Lock()
	defer l.Unlock()

	_, r, err := record.ConsumeEnvelope(peerRecord, peer.PeerRecordEnvelopeDomain)
	if err != nil {
		log.Warnf("error consuming peer record envelope: %s", err)
		return
	}

	rec, ok := r.(*peer.PeerRecord)
	if !ok {
		log.Warnf("bogus peer record for %s; envelope payload is not PeerRecord", p)
		return
	}

	if rec.PeerID != p {
		log.Warnf("bogus peer record for %s; peer ID %s doesn't match peer", p, rec.PeerID)
		return
	}

	seqno, ok := l.peers[p]
	if ok && rec.Seq <= seqno {
		log.Debugf("ignoring peer record for %s; seqno %d is not newer than existing seqno %d", rec.Seq, seqno)
		return
	}

	l.peers[p] = rec.Seq

	err = l.out.Encode(rec)
	if err != nil {
		log.Errorf("error encoding peer record: %s", err)
	}
}
