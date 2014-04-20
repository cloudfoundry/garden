package client

import (
	"time"

	"github.com/cloudfoundry-incubator/garden/client/connection"
)

type connectionPool struct {
	connectionProvider ConnectionProvider
	connections        chan connection.Connection
}

func (pool *connectionPool) Acquire() connection.Connection {
	for {
		select {
		case conn := <-pool.connections:
			select {
			case <-conn.Disconnected():
			default:
				return conn
			}

		default:
			return pool.connect()
		}
	}
}

func (pool *connectionPool) Release(conn connection.Connection) {
	select {
	case pool.connections <- conn:
	default:
		conn.Close()
	}
}

func (pool *connectionPool) connect() connection.Connection {
	for {
		conn, err := pool.connectionProvider.ProvideConnection()
		if err == nil {
			return conn
		}

		time.Sleep(500 * time.Millisecond)
	}
}
