package clickhousegw

import (
	"database/sql"
	"fmt"

	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/pkg/errors"

	"github.com/ClickHouse/clickhouse-go"
)

type ClickHouseGateway struct {
	db *sql.DB
}

func New() (*ClickHouseGateway, error) {
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
		CREATE TABLE IF NOT EXISTS flows (
			agent      IPv6,
			int_in     UInt32,
			int_out    UInt32,
			src_addr   IPv6,
			dst_addr   IPv6,
			protocol   UInt8,
			src_port   UInt16,
			dst_port   UInt16,
			timestamp  DateTime,
			size       UInt64,
			packets    UInt64,
			samplerate UInt64
		) ENGINE = MergeTree()
		PARTITION BY toStartOfTenMinutes(timestamp)
		ORDER BY (timestamp)
		TTL timestamp + INTERVAL 14 DAY
		SETTINGS index_granularity = 8192
	`)

	if err != nil {
		return nil, errors.Wrap(err, "Query failed")
	}

	return &ClickHouseGateway{
		db: c,
	}, nil
}

// Insert inserts flows into clickhouse
func (c *ClickHouseGateway) Insert(flows []*flow.Flow) error {
	tx, err := c.db.Begin()
	if err != nil {
		return errors.Wrap(err, "Begin failed")
	}

	stmt, err := tx.Prepare("INSERT INTO flows (agent, int_in, int_out, src_addr, dst_addr, protocol, src_port, dst_port, timestamp, size, packets, samplerate) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return errors.Wrap(err, "Prepare failed")
	}

	defer stmt.Close()

	for _, fl := range flows {
		_, err := stmt.Exec(
			fl.Agent.ToNetIP(),
			fl.IntIn,
			fl.IntOut,
			fl.SrcAddr.ToNetIP(),
			fl.DstAddr.ToNetIP(),
			fl.Protocol,
			fl.SrcPort,
			fl.DstPort,
			fl.Timestamp,
			fl.Size,
			fl.Packets,
			fl.Samplerate,
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
