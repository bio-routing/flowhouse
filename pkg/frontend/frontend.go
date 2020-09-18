package frontend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/bio-routing/flowhouse/cmd/flowhouse/config"
	"github.com/bio-routing/flowhouse/pkg/clickhousegw"

	log "github.com/sirupsen/logrus"
)

var (
	fields []struct {
		Name  string
		Label string
	}
)

func init() {
	fields = []struct {
		Name  string
		Label string
	}{
		{
			Name:  "agent",
			Label: "Agent",
		},
		{
			Name:  "int_in",
			Label: "Interface In",
		},
		{
			Name:  "int_out",
			Label: "Interface Out",
		},
		{
			Name:  "src_ip_addr",
			Label: "Source IP",
		},
		{
			Name:  "dst_ip_addr",
			Label: "Destination IP",
		},
		{
			Name:  "src_asn",
			Label: "Source ASN",
		},
		{
			Name:  "dst_asn",
			Label: "Destination ASN",
		},
		{
			Name:  "ip_protocol",
			Label: "IP Protocol",
		},
		{
			Name:  "src_port",
			Label: "Source Port",
		},
		{
			Name:  "dst_port",
			Label: "Destination Port",
		},
	}
}

type Frontend struct {
	chgw     *clickhousegw.ClickHouseGateway
	dictCfgs []*config.Dict
}

type Sites []string

type IndexData struct {
	FieldGroups  []*FieldGroup
	BreakDownLen int
}

type FieldGroup struct {
	Name   string
	Label  string
	Fields []*Field
}

type Field struct {
	Name  string
	Label string
}

func New(chgw *clickhousegw.ClickHouseGateway, dictCfgs []*config.Dict) *Frontend {
	return &Frontend{
		chgw:     chgw,
		dictCfgs: dictCfgs,
	}
}

func (fe *Frontend) IndexHandler(w http.ResponseWriter, r *http.Request) {
	templateAsset, err := assetsIndexHtml()
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	t, err := template.New("index.html").Parse(string(templateAsset.bytes))
	if err != nil {
		log.WithError(err).Error("Unable to parse template")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	indexData, err := fe.getIndexData()
	if err != nil {
		log.WithError(err).Error("Unable to get index data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	buf := bytes.NewBuffer(nil)
	err = t.Execute(buf, indexData)
	if err != nil {
		log.WithError(err).Error("Unable to execute template")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(buf.Bytes())
}

func (fe *Frontend) getIndexData() (*IndexData, error) {
	ret := &IndexData{
		FieldGroups: make([]*FieldGroup, 0),
	}

	for _, field := range fields {
		fg := &FieldGroup{
			Name:   field.Name,
			Label:  field.Label,
			Fields: make([]*Field, 0),
		}
		ret.FieldGroups = append(ret.FieldGroups, fg)

		fg.Fields = append(fg.Fields, &Field{
			Name:  field.Name,
			Label: field.Label,
		})

		for _, d := range fe.dictCfgs {
			if d.Field != field.Name {
				continue
			}

			dictFields, err := fe.chgw.DescribeDict(d.Dict)
			if err != nil {
				continue
			}

			for i := 1; i < len(dictFields); i++ {
				fg.Fields = append(fg.Fields, &Field{
					Name:  fmt.Sprintf("%s__%s", field.Name, dictFields[i]),
					Label: fmt.Sprintf("%s %s", field.Label, strings.Title(dictFields[i])),
				})

				ret.BreakDownLen++
			}

		}

		ret.BreakDownLen += 2
	}

	return ret, nil
}

func (fe *Frontend) getFieldsDictName(fieldName string) string {
	for _, d := range fe.dictCfgs {
		if d.Field == fieldName {
			return d.Dict
		}
	}

	return ""
}

// GetDictValues gets a dicts columns values
func (fe *Frontend) GetDictValues(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fieldName, column, err := parseDictValueRequest(parts[2])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	dict := fe.getFieldsDictName(fieldName)
	if dict == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	values, err := fe.chgw.GetDictValues(dict, column)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	res := make(Sites, 0)
	for _, v := range values {
		if v != "" {
			res = append(res, v)
		}
	}

	sort.Slice(res, func(i int, j int) bool {
		return res[i] < res[j]
	})

	j, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(j)
}

func parseDictValueRequest(input string) (string, string, error) {
	parts := strings.Split(input, "__")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid format")
	}

	return parts[0], parts[1], nil
}
