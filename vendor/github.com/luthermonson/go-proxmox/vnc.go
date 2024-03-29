//go:build vnc
// +build vnc

package proxmox

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/buger/goterm"
	"github.com/gorilla/websocket"
)

func (c *Client) VNCWebSocket(path string, vnc *VNC) (chan string, chan string, chan error, func() error, error) {
	if strings.HasPrefix(path, "/") {
		path = strings.Replace(c.baseURL, "https://", "wss://", 1) + path
	}

	var tlsConfig *tls.Config
	transport := c.httpClient.Transport.(*http.Transport)
	if transport != nil {
		tlsConfig = transport.TLSClientConfig
	}
	c.log.Debugf("connecting to websocket: %s", path)
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		TLSClientConfig:  tlsConfig,
	}

	dialerHeaders := http.Header{}
	c.authHeaders(&dialerHeaders)

	conn, _, err := dialer.Dial(path, dialerHeaders)

	if err != nil {
		return nil, nil, nil, nil, err
	}

	// start the session by sending user@realm:ticket
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte(vnc.User+":"+vnc.Ticket+"\n")); err != nil {
		return nil, nil, nil, nil, err
	}

	// it sends back the same thing you just sent so catch it drop it
	_, msg, err := conn.ReadMessage()
	if err != nil || string(msg) != "OK" {
		if err := conn.Close(); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("error closing websocket: %s", err.Error())
		}
		return nil, nil, nil, nil, fmt.Errorf("unable to establish websocket: %s", err.Error())
	}

	type size struct {
		height int
		width  int
	}
	// start the session by sending user@realm:ticket
	tsize := size{
		height: goterm.Height(),
		width:  goterm.Width(),
	}

	c.log.Debugf("sending terminal size: %d x %d", tsize.height, tsize.width)
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("1:%d:%d:", tsize.height, tsize.width))); err != nil {
		return nil, nil, nil, nil, err
	}

	send := make(chan string)
	recv := make(chan string)
	errs := make(chan error)
	done := make(chan struct{})
	ticker := time.NewTicker(30 * time.Second)
	resize := make(chan size)

	go func(tsize size) {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				resized := size{
					height: goterm.Height(),
					width:  goterm.Width(),
				}
				if tsize.height != resized.height ||
					tsize.width != resized.width {
					tsize = resized
					resize <- resized
				}
			}
		}
	}(tsize)

	closer := func() error {
		close(done)
		time.Sleep(1 * time.Second)
		close(send)
		close(recv)
		close(errs)
		ticker.Stop()

		return conn.Close()
	}

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, msg, err := conn.ReadMessage()
				if err != nil {
					if strings.Contains(err.Error(), "use of closed network connection") {
						return
					}
					if !websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						return
					}
					errs <- err
				}
				recv <- string(msg)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-done:
				if err := conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					errs <- err
				}
				return
			case <-ticker.C:
				c.log.Debugf("sending wss keep alive")
				if err := conn.WriteMessage(websocket.BinaryMessage, []byte("2")); err != nil {
					errs <- err
				}
			case resized := <-resize:
				c.log.Debugf("resizing terminal window: %d x %d", resized.height, resized.width)
				if err := conn.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("1:%d:%d:", resized.height, resized.width))); err != nil {
					errs <- err
				}
			case msg := <-send:
				c.log.Debugf("sending: %s", msg)
				m := []byte(msg)
				send := append([]byte(fmt.Sprintf("0:%d:", len(m))), m...)
				if err := conn.WriteMessage(websocket.BinaryMessage, send); err != nil {
					errs <- err
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, []byte("0:1:\n")); err != nil {
					errs <- err
				}
			}
		}
	}()

	return send, recv, errs, closer, nil
}

// VNCWebSocket copy/paste when calling to get the channel names right
// send, recv, errors, closer, errors := vm.VNCWebSocket(vnc)
// for this to work you need to first set up a serial terminal on your vm https://pve.proxmox.com/wiki/Serial_Terminal
func (v *VirtualMachine) VNCWebSocket(vnc *VNC) (chan string, chan string, chan error, func() error, error) {
	p := fmt.Sprintf("/nodes/%s/qemu/%d/vncwebsocket?port=%d&vncticket=%s",
		v.Node, v.VMID, vnc.Port, url.QueryEscape(vnc.Ticket))

	return v.client.VNCWebSocket(p, vnc)
}

func (c *Container) VNCWebSocket(vnc *VNC) (chan string, chan string, chan error, func() error, error) {
	p := fmt.Sprintf("/nodes/%s/lxc/%d/vncwebsocket?port=%d&vncticket=%s",
		c.Node, c.VMID, vnc.Port, url.QueryEscape(vnc.Ticket))

	return c.client.VNCWebSocket(p, vnc)
}

// VNCWebSocket send, recv, errors, closer, error
func (n *Node) VNCWebSocket(vnc *VNC) (chan string, chan string, chan error, func() error, error) {
	p := fmt.Sprintf("/nodes/%s/vncwebsocket?port=%d&vncticket=%s",
		n.Name, vnc.Port, url.QueryEscape(vnc.Ticket))

	return n.client.VNCWebSocket(p, vnc)
}
