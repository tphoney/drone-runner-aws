// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package ssh

import (
	"context"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/drone/runner-go/logger"
)

const networkTimeout = time.Minute * 10

// DialRetry configures and dials the ssh server and
// retries until a connection is established or a timeout
// is reached.
func DialRetry(ctx context.Context, ip, username, privatekey string) (*ssh.Client, error) {
	client, err := Dial(ip, username, privatekey)
	if err == nil {
		return client, nil
	}

	ctx, cancel := context.WithTimeout(ctx, networkTimeout)
	defer cancel()
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		logger.FromContext(ctx).
			WithField("ip", ip).
			WithField("attempt", i).
			Trace("dialing the vm")
		client, err = Dial(ip, username, privatekey)
		if err == nil {
			return client, nil
		}
		logger.FromContext(ctx).
			WithError(err).
			WithField("ip", ip).
			WithField("attempt", i).
			Trace("failed to re-dial vm")

		if client != nil {
			client.Close()
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second * 10):
		}
	}
}

// Dial configures and dials the ssh server.
func Dial(server, username, privatekey string) (*ssh.Client, error) {
	if !strings.HasSuffix(server, ":22") {
		server = server + ":22"
	}
	config := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	pem := []byte(privatekey)
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, err
	}
	config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	return ssh.Dial("tcp", server, config)
}

// func dial(server, username, password string) (*ssh.Client, error) {
// 	return ssh.Dial("tcp", server, &ssh.ClientConfig{
// 		HostKeyCallback: ssh.InsecureIgnoreHostKey(),

// 		User: username,
// 		Auth: []ssh.AuthMethod{
// 			ssh.Password(password),
// 		},
// 	})
// }
