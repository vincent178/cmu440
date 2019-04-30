package p0

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

type client struct {
	conn  net.Conn
	queue chan []byte
	close chan int
}

func (c *client) write(msg []byte) chan bool {
	ret := make(chan bool, 1)
	if _, err := c.conn.Write(msg); err != nil {
		ret <- false
		return ret
	}
	ret <- true
	return ret
}

type multiEchoServer struct {
	ln      net.Listener
	clients map[net.Conn]*client
}

// New creates and returns (but does not start) a new MultiEchoServer.
func New() MultiEchoServer {
	return &multiEchoServer{
		clients: make(map[net.Conn]*client),
	}
}

func (mes *multiEchoServer) Start(port int) error {

	addr := net.JoinHostPort("localhost", strconv.Itoa(port))
	if ln, err := net.Listen("tcp", addr); err != nil {
		fmt.Println("listen port err: ", err)
		return err
	} else {
		mes.ln = ln
	}

	recMsgChan := make(chan []byte)
	delConnChan := make(chan net.Conn)

	go func() {
		for {
			if conn, err := mes.ln.Accept(); err != nil {
				switch err.(type) {
				case *net.OpError:
					return
				default:
					fmt.Println("could't accept: ", err)
					continue
				}
			} else {
				c := &client{
					conn:  conn,
					queue: make(chan []byte, 100),
					close: make(chan int),
				}

				mes.clients[conn] = c

				// add a new goroutine to receive messages and write to conn
				go func() {
					for {
						select {
						case msg := <-c.queue:
							c.conn.Write(msg)
						case <-c.close:
							return
						}
					}
				}()

				go mes.handleConn(c, recMsgChan, delConnChan)
			}
		}
	}()

	go func() {
		for {
			select {
			case msg := <-recMsgChan:
				for _, c := range mes.clients {
					select {
					case c.queue <- msg:
					default:
					}
				}
			case conn := <-delConnChan:
				delete(mes.clients, conn)
			}
		}
	}()

	return nil
}

func (mes *multiEchoServer) Close() {
	mes.ln.Close()
}

func (mes *multiEchoServer) Count() int {
	return len(mes.clients)
}

func (mes *multiEchoServer) handleConn(c *client, recMsgChan chan<- []byte, delConnChan chan<- net.Conn) {
	b := bufio.NewReader(c.conn)

	for {
		if line, err := b.ReadBytes('\n'); err != nil {
			if err == io.EOF {
				c.conn.Close()
				c.close <- 1
				delConnChan <- c.conn
				return
			} else {
				fmt.Println("error read bytes from conn: ", err)
			}
		} else {
			recMsgChan <- line
		}
		time.Sleep(1 * time.Millisecond)
	}
}
