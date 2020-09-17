package config

import (
	"fmt"
	"io/ioutil"
	"net"

	"github.com/bio-routing/bio-rd/routingtable/vrf"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	listenSFlowDefault = ":6343"
	listenHTTPDefault  = ":9991"
)

// Config represents a config file
type Config struct {
	RISTimeout  uint64      `yaml:"ris_timeout"`
	ListenSFlow string      `yaml:"listen_sflow"`
	ListenHTTP  string      `yaml:"listen_http"`
	Routers     []*Router   `yaml:"routers"`
	Clickhouse  *Clickhouse `yaml:"clickhouse"`
}

// Clickhouse represents a clickhouse client config
type Clickhouse struct {
	Host     string `yaml:"host"`
	Address  string `yaml:"address"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

func (c *Config) load() error {
	if c.RISTimeout == 0 {
		c.RISTimeout = 10
	}

	if c.ListenSFlow == "" {
		c.ListenSFlow = listenSFlowDefault
	}

	if c.ListenHTTP == "" {
		c.ListenHTTP = listenHTTPDefault
	}

	for _, r := range c.Routers {
		err := r.load()
		if err != nil {
			return errors.Wrapf(err, "Unable to load config for router %q", r.Name)
		}
	}

	return nil
}

// Router represents a router
type Router struct {
	Name         string `yaml:"name"`
	Address      string `yaml:"address"`
	address      net.IP
	RISInstances []string `yaml:"ris_instances"`
	VRFs         []string `yaml:"vrfs"`
	vrfs         []uint64
}

// GetAddress gets a routers address
func (r *Router) GetAddress() net.IP {
	return r.address
}

// GetVRFs gets a routers VRFs
func (r *Router) GetVRFs() []uint64 {
	return r.vrfs
}

func (r *Router) load() error {
	a := net.ParseIP(r.Address)
	if a == nil {
		return fmt.Errorf("Invalid router IP address %q", r.Address)
	}

	r.address = a

	for _, x := range r.VRFs {
		vrfRD, err := vrf.ParseHumanReadableRouteDistinguisher(x)
		if err != nil {
			return errors.Wrapf(err, "Unable to parse VRF RD %q", x)
		}

		r.vrfs = append(r.vrfs, vrfRD)
	}

	return nil
}

// GetConfig gets the configuration
func GetConfig(fp string) (*Config, error) {
	fc, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read file")
	}

	c := &Config{}
	err = yaml.Unmarshal(fc, c)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to unmarshal")
	}

	c.load()

	return c, nil
}

// GetRISList gets a list of all referenced RIS instances
func (c *Config) GetRISList() []string {
	m := make(map[string]struct{})

	for _, rtr := range c.Routers {
		for _, x := range rtr.RISInstances {
			m[x] = struct{}{}
		}
	}

	ret := make([]string, 0)
	for k := range m {
		ret = append(ret, k)
	}

	return ret
}
