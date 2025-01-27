package config

import (
	"io/ioutil"

	"github.com/bio-routing/bio-rd/routingtable/vrf"
	"github.com/bio-routing/flowhouse/pkg/clickhousegw"
	"github.com/bio-routing/flowhouse/pkg/frontend"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	bnet "github.com/bio-routing/bio-rd/net"
)

const (
	listenSFlowDefault = ":6343"
	listenHTTPDefault  = ":9991"
)

// Config represents a config file
type Config struct {
	RISTimeout  uint64      `yaml:"ris_timeout"`
	SNMP        *SNMPConfig `yaml:"snmp"`
	DefaultVRF  string      `yaml:"default_vrf"`
	defaultVRF  uint64
	ListenSFlow string                         `yaml:"listen_sflow"`
	ListenIPFIX string                         `yaml:"listen_ipfix"`
	ListenHTTP  string                         `yaml:"listen_http"`
	Dicts       frontend.Dicts                 `yaml:"dicts"`
	Clickhouse  *clickhousegw.ClickhouseConfig `yaml:"clickhouse"`
	Routers     []*Router                      `yaml:"routers"`
}

type SNMPConfig struct {
	Version           uint   `yaml:"version"`
	Community         string `yaml:"community"`
	User              string `yaml:"user"`
	AuthPassphrase    string `yaml:"auth-key"`
	PrivacyPassphrase string `yaml:"privacy-passphrase"`
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

	if c.DefaultVRF != "" {
		vrfID, err := vrf.ParseHumanReadableRouteDistinguisher(c.DefaultVRF)
		if err != nil {
			return errors.Wrap(err, "Unable to perse default VRF")
		}

		c.defaultVRF = vrfID
	}

	for _, r := range c.Routers {
		err := r.load()
		if err != nil {
			return errors.Wrapf(err, "Unable to load config for router %q", r.Name)
		}
	}

	return nil
}

// GetDefaultVRF gets the default VRF id
func (c *Config) GetDefaultVRF() uint64 {
	return c.defaultVRF
}

// Router represents a router
type Router struct {
	Name         string `yaml:"name"`
	Address      string `yaml:"address"`
	address      bnet.IP
	RISInstances []string `yaml:"ris_instances"`
	VRFs         []string `yaml:"vrfs"`
	vrfs         []uint64
}

// GetAddress gets a routers address
func (r *Router) GetAddress() bnet.IP {
	return r.address
}

// GetVRFs gets a routers VRFs
func (r *Router) GetVRFs() []uint64 {
	return r.vrfs
}

func (r *Router) load() error {
	a, err := bnet.IPFromString(r.Address)
	if err != nil {
		return errors.Wrap(err, "Unable to parse IP address")
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
