package ifnames

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/ClickHouse/clickhouse-go"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/ifnamecollector"
)

type IfNamesSaver struct {
	db *sql.DB
}

func New(host string, port uint16, user string, password string) (*IfNamesSaver, error) {
	c, err := sql.Open("clickhouse", "tcp://localhost:9000?username=default&password=V3aabyEm78N4LQU5&database=flows&read_timeout=10&write_timeout=20")
	if err := c.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		} else {
			fmt.Println(err)
		}
		return nil, err
	}

	_, err = c.Exec(`
		CREATE TABLE IF NOT EXISTS ifnames (
			agent      IPv6,
			id         UInt32,
			name       String,
			rdev       String,
			rif        String,
			ri         String,
			role       String,
			timestamp  DateTime
		) ENGINE = MergeTree()
		PARTITION BY toYYYYMMDD(timestamp)
		ORDER BY (timestamp)
		TTL timestamp + INTERVAL 365 DAY
		SETTINGS index_granularity = 8192
	`)

	if err != nil {
		return nil, errors.Wrap(err, "Query failed")
	}

	return &IfNamesSaver{
		db: c,
	}, nil
}

// Insert inserts interface names into clickhouse
func (c *IfNamesSaver) Insert(agent bnet.IP, m ifnamecollector.InterfaceIDByName) error {
	tx, err := c.db.Begin()
	if err != nil {
		return errors.Wrap(err, "Begin failed")
	}

	stmt, err := tx.Prepare("INSERT INTO ifnames (agent, id, name, rdev, rif, ri, role, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return errors.Wrap(err, "Prepare failed")
	}

	defer stmt.Close()

	agentNetIP := agent.ToNetIP()
	now := time.Now().Unix()

	for k, v := range m {
		_, err := stmt.Exec(
			agentNetIP,
			k,
			v.Name,
			v.RemoteDevice,
			v.RemoteInterface,
			v.RoutingInstance,
			v.Role,
			now,
		)
		if err != nil {
			return errors.Wrap(err, "Exec failed")
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "Commit failed")
	}

	return nil
}
