/**
 * File        : ping.go
 * Description : Service for testing the reachability of peers.
 * Copyright   : Copyright (c) 2017-2018 DFINITY Stiftung. All rights reserved.
 * Maintainer  : Enzo Haussecker <enzo@dfinity.org>
 * Stability   : Experimental
 */

package p2p

import (
	"bytes"
	"crypto/rand"
	"errors"
	"time"

	"gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"

	"github.com/dfinity/go-revolver/util"
)

// Ping a peer.
func (client *client) ping(peerId peer.ID) error {

	// Log this action.
	pid := peerId
	client.logger.Debug("Ping", pid)

	// Connect to the target peer.
	stream, err := client.host.NewStream(
		client.context,
		pid,
		client.protocol+"/ping",
	)
	if err != nil {
		addrs := client.peerstore.PeerInfo(pid).Addrs
		client.logger.Debug("Cannot connect to", pid, "at", addrs, err)
		client.peerstore.ClearAddrs(pid)
		client.table.Remove(pid)
		return err
	}
	defer stream.Close()

	// Generate random data.
	wbuf := make([]byte, 32)
	_, err = rand.Reader.Read(wbuf)
	if err != nil {
		client.logger.Warning("Cannot generate random data", err)
		return err
	}

	// Observe the current time.
	before := time.Now()

	// Send data to the target peer.
	err = util.WriteWithTimeout(stream, wbuf, client.config.Timeout)
	if err != nil {
		client.logger.Warning("Cannot send data to", pid, err)
		return err
	}

	// Receive data from the target peer.
	rbuf, err := util.ReadWithTimeout(stream, 32, client.config.Timeout)
	if err != nil {
		client.logger.Warning("Cannot receive data from", pid, err)
		return err
	}

	// Verify that the data sent and received is the same.
	if !bytes.Equal(wbuf, rbuf) {
		err = errors.New("Corrupt data!")
		client.logger.Warning("Cannot verify data received from", pid, err)
		return err
	}

	// Record the observed latency.
	client.peerstore.RecordLatency(pid, time.Since(before))
	client.peerstore.Put(pid, "PINGED_AT", time.Now())

	// Success.
	return nil

}

// Handle incomming pings.
func (client *client) pingHandler(stream net.Stream) {

	defer stream.Close()

	// Log this action.
	pid := stream.Conn().RemotePeer()
	client.logger.Debug("Pong", pid)

	// Check if the target peer is authorized to perform this action.
	authorized, err := client.peerstore.Get(pid, "AUTHORIZED")
	if err != nil || !authorized.(bool) {
		client.logger.Warning("Unauthorized request from", pid)
		return
	}

	// Receive data from the target peer.
	rbuf, err := util.ReadWithTimeout(stream, 32, client.config.Timeout)
	if err != nil {
		client.logger.Warning("Cannot receive data from", pid, err)
		return
	}

	// Send data to the target peer.
	err = util.WriteWithTimeout(stream, rbuf, client.config.Timeout)
	if err != nil {
		client.logger.Warning("Cannot send data to", pid, err)
	}

}

// Register the ping handler.
func (client *client) registerPingService() {
	uri := client.protocol + "/ping"
	client.host.SetStreamHandler(uri, client.pingHandler)
}
