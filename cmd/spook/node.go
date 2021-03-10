package main

import (
	"context"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-noise"
	"github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	tls "github.com/libp2p/go-libp2p-tls"
	"github.com/libp2p/go-msgio/protoio"

	ma "github.com/multiformats/go-multiaddr"
)

const HelloProtocolID = "/fil/hello/1.0.0"

var bootstrappers = []string{
	"/dns4/bootstrap-0.mainnet.filops.net/tcp/1347/p2p/12D3KooWCVe8MmsEMes2FzgTpt9fXtmCY7wrq91GRiaC8PHSCCBj",
	"/dns4/bootstrap-1.mainnet.filops.net/tcp/1347/p2p/12D3KooWCwevHg1yLCvktf2nvLu7L9894mcrJR4MsBCcm4syShVc",
	"/dns4/bootstrap-2.mainnet.filops.net/tcp/1347/p2p/12D3KooWEWVwHGn2yR36gKLozmb4YjDJGerotAPGxmdWZx2nxMC4",
	"/dns4/bootstrap-3.mainnet.filops.net/tcp/1347/p2p/12D3KooWKhgq8c7NQ9iGjbyK7v7phXvG6492HQfiDaGHLHLQjk7R",
	"/dns4/bootstrap-4.mainnet.filops.net/tcp/1347/p2p/12D3KooWL6PsFNPhYftrJzGgF5U18hFoaVhfGk7xwzD8yVrHJ3Uc",
	"/dns4/bootstrap-5.mainnet.filops.net/tcp/1347/p2p/12D3KooWLFynvDQiUpXoHroV1YxKHhPJgysQGH2k3ZGwtWzR4dFH",
	"/dns4/bootstrap-6.mainnet.filops.net/tcp/1347/p2p/12D3KooWP5MwCiqdMETF9ub1P3MbCvQCcfconnYHbWg6sUJcDRQQ",
	"/dns4/bootstrap-7.mainnet.filops.net/tcp/1347/p2p/12D3KooWRs3aY1p3juFjPy8gPN95PEQChm2QKGUCAdcDCC4EBMKf",
	"/dns4/bootstrap-8.mainnet.filops.net/tcp/1347/p2p/12D3KooWScFR7385LTyR4zU1bYdzSiiAb5rnNABfVahPvVSzyTkR",
	// "/dns4/lotus-bootstrap.forceup.cn/tcp/41778/p2p/12D3KooWFQsv3nRMUevZNWWsY1Wu6NUzUbawnWU5NcRhgKuJA37C",
	// "/dns4/bootstrap-0.starpool.in/tcp/12757/p2p/12D3KooWGHpBMeZbestVEWkfdnC9u7p6uFHXL1n7m1ZBqsEmiUzz",
	// "/dns4/bootstrap-1.starpool.in/tcp/12757/p2p/12D3KooWQZrGH1PxSNZPum99M1zNvjNFM33d1AAu5DcvdHptuU7u",
	// "/dns4/node.glif.io/tcp/1235/p2p/12D3KooWBF8cpp65hp2u9LK5mh19x67ftAam84z9LsfaquTDSBpt",
	// "/dns4/bootstrap-0.ipfsmain.cn/tcp/34721/p2p/12D3KooWQnwEGNqcM2nAcPtRR9rAX8Hrg4k9kJLCHoTR5chJfz6d",
	// "/dns4/bootstrap-1.ipfsmain.cn/tcp/34723/p2p/12D3KooWMKxMkD5DMpSWsW7dBddKxKT7L2GgbNuckz9otxvkvByP",
}

type Node struct {
	host   host.Host
	logger *Logger

	sync.Mutex
	nextGraft map[peer.ID]time.Time
}

func NewNode(l *Logger, idFile string) (*Node, error) {
	opts := []libp2p.Option{
		libp2p.DisableRelay(),
		libp2p.NoListenAddrs,
		libp2p.Security(noise.ID, noise.New),
		libp2p.Security(tls.ID, tls.New),
	}

	if idFile != "" {
		privk, err := loadIdentity(idFile)
		if err != nil {
			return nil, err
		}
		opts = append(opts, libp2p.Identity(privk))
	}

	h, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	n := &Node{
		host:      h,
		logger:    l,
		nextGraft: make(map[peer.ID]time.Time),
	}
	h.SetStreamHandler(pubsub.GossipSubID_v11, n.handleGSStream)
	//h.SetStreamHandler(HelloProtocolID, n.handleHelloStream)

	return n, nil
}

func (n *Node) Background(wg *sync.WaitGroup) {
	boot := 0
	for i, a := range bootstrappers {
		addr, err := ma.NewMultiaddr(a)
		if err != nil {
			log.Errorf("error parsing bootstrapper %d address: %s", i, err)
			continue
		}

		pi, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			log.Errorf("error parsing bootstrapper %d address: %s", i, err)
			continue
		}

		go n.connect(pi)
		boot++
	}

	if boot == 0 {
		wg.Done()
	}
}

func (n *Node) connect(pi *peer.AddrInfo) {
	for {
		log.Debugf("connecting to bootstrapper %s", pi.ID)

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := n.host.Connect(ctx, *pi)
		cancel()
		if err != nil {
			log.Warnf("error connecting to bootstrapper %s: %s", pi.ID, err)
			time.Sleep(time.Minute)
			continue
		}

		log.Debugf("connected to bootstrapper %s", pi.ID)

		for n.host.Network().Connectedness(pi.ID) == network.Connected {
			time.Sleep(5 * time.Minute)
		}
	}
}

func (n *Node) handleGSStream(in network.Stream) {
	defer in.Close()

	p := in.Conn().RemotePeer()
	defer n.host.Network().ClosePeer(p)

	log.Debugf("inbound stream from %s; opening outbound stream", p)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	out, err := n.host.NewStream(ctx, p, pubsub.GossipSubID_v11)
	cancel()

	if err != nil {
		log.Errorf("error opening outbound stream to %s: %s", p, err)
		return
	}

	defer out.Close()

	r := protoio.NewDelimitedReader(in, 1<<20)
	w := protoio.NewDelimitedWriter(out)

	log.Debugf("reading hello packet from %s", p)
	var rpc pb.RPC
	err = r.ReadMsg(&rpc)
	if err != nil {
		log.Warnf("error reading RPC from %s: %s", err)
		in.Reset()
		out.Reset()
		return
	}

	t := true
	var subs []*pb.RPC_SubOpts
	var graft []*pb.ControlGraft
	for _, sub := range rpc.GetSubscriptions() {
		topic := new(string)
		*topic = sub.GetTopicid()
		log.Debugf("peer %s is subscribed to %s", p, *topic)
		subs = append(subs, &pb.RPC_SubOpts{
			Subscribe: &t,
			Topicid:   topic,
		})
		graft = append(graft, &pb.ControlGraft{TopicID: topic})
	}

	n.Lock()
	nextGraft, ok := n.nextGraft[p]
	n.Unlock()

	now := time.Now()
	if ok && now.Before(nextGraft) {
		wait := nextGraft.Sub(now) + time.Second
		log.Debugf("waiting %s for next graft", wait)
		time.Sleep(wait)
	}

	rpc.Reset()
	//rpc.Subscriptions = subs
	rpc.Control = &pb.ControlMessage{Graft: graft}
	log.Debugf("writing hello packet to %s", p)
	err = w.WriteMsg(&rpc)
	if err != nil {
		log.Warnf("error writing RPC to %s: %s", err)
		in.Reset()
		out.Reset()
		return
	}

	for {
		rpc.Reset()
		err = r.ReadMsg(&rpc)
		if err != nil {
			if err != io.EOF {
				log.Warnf("error reading RPC from %s: %s", p, err)
			}
			in.Reset()
			out.Reset()
			return
		}

		ctl := rpc.GetControl()
		if ctl == nil {
			continue
		}

		for _, prune := range ctl.GetPrune() {
			topic := prune.GetTopicID()
			peers := prune.GetPeers()
			backoff := prune.GetBackoff()

			log.Debugf("got pruned by %s in %s; PX returned %d peers", p, topic, len(peers))

			for _, pi := range peers {
				spr := pi.GetSignedPeerRecord()
				if spr != nil {
					n.logger.AddPeer(peer.ID(pi.GetPeerID()), spr)
				}
			}

			wait := time.Duration(backoff) * time.Second
			nextGraft := time.Now().Add(wait)
			n.Lock()
			n.nextGraft[p] = nextGraft
			n.Unlock()
			go func(topic string) {
				time.Sleep(wait + time.Duration(1+rand.Intn(10))*time.Second)
				rpc := &pb.RPC{
					Control: &pb.ControlMessage{
						Graft: []*pb.ControlGraft{
							&pb.ControlGraft{
								TopicID: &topic,
							},
						},
					},
				}

				err = w.WriteMsg(rpc)
				if err != nil {
					log.Warnf("error writing RPC to %s: %s", p, err)
				}
			}(topic)
		}
	}
}

func (n *Node) handleHelloStream(s network.Stream) {
	defer s.Close()

	io.Copy(s, s)
}
