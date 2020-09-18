package clickhousegw

import (
	"database/sql"
	"fmt"
	"net"
	"strings"

	"github.com/bio-routing/flowhouse/cmd/flowhouse/config"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/pkg/errors"

	"github.com/ClickHouse/clickhouse-go"

	bnet "github.com/bio-routing/bio-rd/net"
)

// ClickHouseGateway is a wrapper for Clickhouse
type ClickHouseGateway struct {
	db *sql.DB
}

// New instantiates a new ClickHouseGateway
func New(cfg *config.Clickhouse) (*ClickHouseGateway, error) {
	dsn := fmt.Sprintf("tcp://%s?username=%s&password=%s&database=%s&read_timeout=10&write_timeout=20", cfg.Address, cfg.User, cfg.Password, cfg.Database)
	c, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "sql.Open failed")
	}

	err = c.Ping()
	if err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			return nil, errors.Wrapf(err, "[%d] %s \n%s", exception.Code, exception.Message, exception.StackTrace)
		}

		return nil, errors.Wrap(err, "c.Ping failed")
	}

	chgw := &ClickHouseGateway{
		db: c,
	}

	err = chgw.createFlowsSchemaIfNotExists()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create flows schema")
	}

	err = chgw.createInterfacesSchemaIfNotExists()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create interfaces schema")
	}

	return chgw, nil
}

func (c *ClickHouseGateway) createInterfacesSchemaIfNotExists() error {
	_, err := c.db.Exec(`
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
		return errors.Wrap(err, "Query failed")
	}

	return nil
}

func (c *ClickHouseGateway) createFlowsSchemaIfNotExists() error {
	_, err := c.db.Exec(`
		CREATE TABLE IF NOT EXISTS flows (
			agent           IPv6,
			int_in          UInt32,
			int_out         UInt32,
			src_ip_addr     IPv6,
			dst_ip_addr     IPv6,
			src_prefix_addr IPv6,
			src_prefix_len  UInt8,
			dst_prefix_addr IPv6,
			dst_prefix_len  UInt8,
			src_asn         UInt32,
			dst_asn         UInt32,
			ip_protocol     UInt8,
			src_port        UInt16,
			dst_port        UInt16,
			timestamp       DateTime,
			size            UInt64,
			packets         UInt64,
			samplerate      UInt64
		) ENGINE = MergeTree()
		PARTITION BY toStartOfTenMinutes(timestamp)
		ORDER BY (timestamp)
		TTL timestamp + INTERVAL 14 DAY
		SETTINGS index_granularity = 8192
	`)

	if err != nil {
		return errors.Wrap(err, "Query failed")
	}

	return nil
}

// InsertFlows inserts flows into clickhouse
func (c *ClickHouseGateway) InsertFlows(flows []*flow.Flow) error {
	tx, err := c.db.Begin()
	if err != nil {
		return errors.Wrap(err, "Begin failed")
	}

	stmt, err := tx.Prepare("INSERT INTO flows (agent, int_in, int_out, src_ip_addr, dst_ip_addr, src_prefix_addr, src_prefix_len, dst_prefix_addr, dst_prefix_len, src_asn, dst_asn, ip_protocol, src_port, dst_port, timestamp, size, packets, samplerate) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
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
			addrToNetIP(fl.SrcPfx.Addr()),
			fl.SrcPfx.Pfxlen(),
			addrToNetIP(fl.DstPfx.Addr()),
			fl.DstPfx.Pfxlen(),
			fl.SrcAs,
			fl.DstAs,
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

func addrToNetIP(addr *bnet.IP) net.IP {
	if addr == nil {
		return net.IP([]byte{0, 0, 0, 0})
	}

	return addr.ToNetIP()
}

// Close closes the database handler
func (c *ClickHouseGateway) Close() {
	c.db.Close()
}

func (c *ClickHouseGateway) GetColumnValues(columnName string) ([]string, error) {
	columnName = strings.Replace(columnName, " ", "", -1)

	query := fmt.Sprintf("SELECT %s FROM flows GROUP BY %s", columnName, columnName)
	res, err := c.db.Query(query)

	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)

	for {
		v := ""
		res.Scan(&v)

		result = append(result, v)
		if !res.Next() {
			break
		}
	}

	return result, nil
}

// GetDictValues gets all values of a certain dicts attribute
func (c *ClickHouseGateway) GetDictValues(dictName string, attr string) ([]string, error) {
	dictName = strings.Replace(dictName, " ", "", -1)
	attr = strings.Replace(attr, " ", "", -1)

	query := fmt.Sprintf("SELECT %s FROM dictionaries.%s GROUP BY %s", attr, dictName, attr)
	res, err := c.db.Query(query)

	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)

	for {
		v := ""
		res.Scan(&v)

		result = append(result, v)
		if !res.Next() {
			break
		}
	}

	return result, nil
}

// DescribeDict gets the names of all fields in a dictionary
func (c *ClickHouseGateway) DescribeDict(dictName string) ([]string, error) {
	dictName = strings.Replace(dictName, " ", "", -1)

	query := fmt.Sprintf("DESCRIBE dictionaries.%s", dictName)
	res, err := c.db.Query(query)

	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)

	for res.Next() {
		name := ""
		trash := ""
		res.Scan(&name, &trash, &trash, &trash, &trash, &trash, &trash)

		result = append(result, name)
	}

	return result, nil
}

// DescribeTable gets the names of all fields of a table
func (c *ClickHouseGateway) DescribeTable(tableName string) ([]string, error) {
	tableName = strings.Replace(tableName, " ", "", -1)

	query := fmt.Sprintf("DESCRIBE %s", tableName)
	res, err := c.db.Query(query)

	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)

	for {
		name := ""
		trash := ""
		res.Scan(&name, &trash, &trash, &trash, &trash, &trash, &trash)

		result = append(result, name)
		if !res.Next() {
			break
		}
	}

	return result, nil
}
