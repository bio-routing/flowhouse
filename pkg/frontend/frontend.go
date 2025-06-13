package frontend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bio-routing/flowhouse/pkg/clickhousegw"
	"github.com/pkg/errors"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	bnet "github.com/bio-routing/bio-rd/net"
	log "github.com/sirupsen/logrus"
)

var (
	fields []struct {
		Name       string
		Label      string
		ShortLabel string
	}
)

func init() {
	fields = []struct {
		Name       string
		Label      string
		ShortLabel string
	}{
		{
			Name:       "agent",
			Label:      "Agent",
			ShortLabel: "A.",
		},
		{
			Name:       "int_in",
			Label:      "Interface In",
			ShortLabel: "Int.In",
		},
		{
			Name:       "int_out",
			Label:      "Interface Out",
			ShortLabel: "Int.Out",
		},
		{
			Name:       "tos",
			Label:      "Type of Service",
			ShortLabel: "TOS",
		},
		{
			Name:       "dscp",
			Label:      "Differentiated Services Code Point",
			ShortLabel: "DSCP",
		},
		{
			Name:       "src_ip_addr",
			Label:      "Source IP",
			ShortLabel: "Src.IP",
		},
		{
			Name:       "src_ip_pfx",
			Label:      "Source IP Prefix",
			ShortLabel: "Src.IP.Pfx",
		},
		{
			Name:       "dst_ip_addr",
			Label:      "Destination IP",
			ShortLabel: "Dst.IP",
		},
		{
			Name:       "dst_ip_pfx",
			Label:      "Destination IP Prefix",
			ShortLabel: "Dst.IP.Pfx",
		},
		{
			Name:       "nexthop",
			Label:      "Nexthop",
			ShortLabel: "Nexthop",
		},
		{
			Name:       "next_asn",
			Label:      "Next ASN",
			ShortLabel: "Next ASN",
		},
		{
			Name:       "src_asn",
			Label:      "Source ASN",
			ShortLabel: "Src.AS",
		},
		{
			Name:       "dst_asn",
			Label:      "Destination ASN",
			ShortLabel: "Dst.AS",
		},
		{
			Name:       "ip_protocol",
			Label:      "IP Protocol",
			ShortLabel: "IP.Proto",
		},
		{
			Name:       "src_port",
			Label:      "Source Port",
			ShortLabel: "Src.Port",
		},
		{
			Name:       "dst_port",
			Label:      "Destination Port",
			ShortLabel: "Dst.Port",
		},
	}
}

// Frontend is a web frontend service
type Frontend struct {
	chgw     *clickhousegw.ClickHouseGateway
	dictCfgs Dicts
}

// IndexView is the index template data structure
type IndexView struct {
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

// Dict connects a fields with a dict
type Dict struct {
	Field string `yaml:"field"`
	Dict  string `yaml:"dict"`
	Expr  string `yaml:"expr"`
}

func (d Dicts) getDict(field string) *Dict {
	for _, x := range d {
		if x.Field == field {
			return x
		}
	}

	return nil
}

// Dicts is a slice of dicts
type Dicts []*Dict

// New creates a new frontend
func New(chgw *clickhousegw.ClickHouseGateway, dictCfgs Dicts) *Frontend {
	return &Frontend{
		chgw:     chgw,
		dictCfgs: dictCfgs,
	}
}

// IndexHandler handles requests for /
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

	indexData, err := fe.getIndexView()
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

// FlowhouseJSHandler gets flowhouse.js file
func (fe *Frontend) FlowhouseJSHandler(w http.ResponseWriter, r *http.Request) {
	jsAsset, err := assetsFlowhouseJs()
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Write(jsAsset.bytes)
}

// QueryHandler handles query requests
func (fe *Frontend) QueryHandler(flatCSV bool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := fe.processQuery(r)
		if err != nil {
			log.WithError(err).Error("Unable to process query")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if res == nil {
			log.WithError(err).Error("Query returned a nil result")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if flatCSV {
			err = res.csvFlat(w)
		} else {
			err = res.csv(w)
		}
		if err != nil {
			log.WithError(err).Errorf("Unable to write CSV")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (fe *Frontend) processQuery(r *http.Request) (*result, error) {
	if len(r.URL.Query()) == 0 {
		return nil, nil
	}

	query, err := fe.fieldsToQuery(r.URL.Query())
	if err != nil {
		return nil, errors.Wrap(err, "Unable to generate SQL query")
	}

	log.Info(query)

	rows, err := fe.chgw.Query(query)
	defer rows.Close()
	if err != nil {
		return nil, errors.Wrap(err, "Query failed")
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get columns")
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}
	res := newResult()

	rowLimit := 500 // default limit of processed rows to avoid OOM (too much data hangs the frontend)
	if topFlowsValues, ok := r.URL.Query()["topFlows"]; ok && len(topFlowsValues) > 0 {
		topFlowsInt, err := strconv.Atoi(topFlowsValues[0])
		if err == nil && topFlowsInt > 0 && topFlowsInt <= 10000 {
			rowLimit = topFlowsInt
		} else {
			log.Errorf("Invalid topFlows value: %v", topFlowsValues[0])
		}
	}
	log.Infof("Top %d rows shown", rowLimit)
	othersData := make(map[time.Time]uint64) // remaining rows are aggregated in othersData[timestamp] = mbps

	rowCount := 0
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, errors.Wrap(err, "Scan failed")
		}

		ts, ok := (*valuePtrs[0].(*interface{})).(time.Time)
		if !ok {
			return nil, fmt.Errorf("expected time.Time for the first column")
		}
		value, ok := (*valuePtrs[len(columns)-1].(*interface{})).(float64)
		if !ok {
			return nil, fmt.Errorf("expected float64 for the last column")
		}

		if rowCount < rowLimit { // Process the top flows normally (sorted by mbps descending)
			keyComponents := make([]string, 0)
			for i := 1; i < len(columns)-1; i++ {
				label := getReadableLabel(columns[i])

				switch (*valuePtrs[i].(*interface{})).(type) {
				case uint8:
					keyComponents = append(keyComponents, fmt.Sprintf("%s=%d", label, (*valuePtrs[i].(*interface{})).(uint8)))
				case uint16:
					keyComponents = append(keyComponents, fmt.Sprintf("%s=%d", label, (*valuePtrs[i].(*interface{})).(uint16)))
				case uint32:
					keyComponents = append(keyComponents, fmt.Sprintf("%s=%d", label, (*valuePtrs[i].(*interface{})).(uint32)))
				case uint64:
					keyComponents = append(keyComponents, fmt.Sprintf("%s=%d", label, (*valuePtrs[i].(*interface{})).(uint64)))
				case string:
					s := (*valuePtrs[i].(*interface{})).(string)
					if strings.Contains(s, "::ffff:") && strings.Contains(s, "/") {
						s = formatPrefix(s)
					}

					keyComponents = append(keyComponents, fmt.Sprintf("%s=%s", label, s))
				case net.IP:
					keyComponents = append(keyComponents, fmt.Sprintf("%s=%s", label, (*valuePtrs[i].(*interface{})).(net.IP).String()))
				}
			}

			res.add(ts, strings.Join(keyComponents, ";"), uint64(value))
		} else { // Aggregate the remaining flows in "Others"
			othersData[ts] += uint64(value)
		}

		rowCount++
	}

	for ts, mbps := range othersData {
		res.add(ts, "Others", mbps)
	}

	return res, nil
}

func formatPrefix(s string) string {
	parts := strings.Split(s, "/")
	addr := net.ParseIP(parts[0])
	return fmt.Sprintf("%s/%s", addr.String(), parts[1])
}

func getReadableLabel(label string) string {
	for _, f := range fields {
		if strings.HasPrefix(label, f.Name) {
			label = strings.Replace(label, f.Name, f.ShortLabel, 1)
			break
		}
	}

	if !strings.Contains(label, "__") {
		return label
	}

	parts := strings.Split(label, "__")
	return fmt.Sprintf("%s.%s", parts[0], strings.Title(parts[1]))
}

func (fe *Frontend) fieldsToQuery(fields url.Values) (string, error) {
	if _, exists := fields["breakdown"]; !exists {
		return "", fmt.Errorf("no breakdown set")
	}

	if _, exists := fields["time_start"]; !exists {
		return "", fmt.Errorf("no start time given")
	}

	if _, exists := fields["time_end"]; !exists {
		return "", fmt.Errorf("no end time given")
	}

	start, err := timeFieldToTimestamp(fields["time_start"][0])
	if err != nil {
		return "", errors.Wrap(err, "Unable to parse time")
	}

	end, err := timeFieldToTimestamp(fields["time_end"][0])
	if err != nil {
		return "", errors.Wrap(err, "Unable to parse time")
	}

	selectFieldList := make([]string, 0)
	selectFieldList = append(selectFieldList, "timestamp as t")
	for _, fieldName := range fields["breakdown"] {
		resolvedFieldName := resolveVirtualField(fieldName)
		statement, err := fe.resolveDictIfNecessary(resolvedFieldName)
		if err != nil {
			log.WithError(err).Warning("Unable to resolve dict. Ignoring selection")
			continue
		}

		selectFieldList = append(selectFieldList, fmt.Sprintf("%s as %s", statement, fieldName))
	}
	selectFieldList = append(selectFieldList, "sum(size * samplerate) * 8 / 10 / 1000000 AS mbps")

	conditions := make([]string, 0)
	conditions = append(conditions, fmt.Sprintf("t BETWEEN toDateTime(%d) AND toDateTime(%d)", start, end))
	for fieldName := range fields {
		if fieldName == "breakdown" || fieldName == "time_start" || fieldName == "time_end" || strings.HasPrefix(fieldName, "filter_field") || fieldName == "topFlows" {
			continue
		}

		statement, err := fe.resolveDictIfNecessary(fieldName)
		if err != nil {
			log.WithError(err).Warning("Unable to resolve dict. Ignoring condition")
			continue
		}

		conditions = append(conditions, formatCondition(statement, fields, fieldName))
	}

	groupBy := make([]string, 0)
	groupBy = append(groupBy, "t")
	if breakdown, ok := fields["breakdown"]; ok {
		groupBy = append(groupBy, breakdown...)
	}

	q := "SELECT %s FROM %s.flows WHERE %s GROUP BY %s ORDER BY mbps DESC LIMIT 10000"
	return fmt.Sprintf(q, strings.Join(selectFieldList, ", "), fe.chgw.GetDatabaseName(), strings.Join(conditions, " AND "), strings.Join(groupBy, ", ")), nil
}

func formatCondition(statement string, fields url.Values, fieldName string) string {
	// TODO: Add support for filtering by Prefix (Dst/Src)

	if len(fields[fieldName]) == 1 {
		return formatConditionSingleValue(statement, fields, fieldName)
	}

	return formatConditionMultiValues(statement, fields, fieldName)
}

func formatConditionSingleValue(statement string, fields url.Values, fieldName string) string {
	v := fields[fieldName][0]
	if isIPField(fieldName) {
		v = formatIPCondition(v)
	} else if isPrefixField(fieldName) {
		var err error
		v, err = formatPrefixCondition(fieldName, v)
		if err != nil {
			return ""
		}
		return v
	} else {
		v = fmt.Sprintf("'%s'", v)
	}

	return fmt.Sprintf("%s = %s", statement, v)
}

func formatConditionMultiValues(statement string, fields url.Values, fieldName string) string {
	if isPrefixField(fieldName) {
		return prefixMultiValueCondition(statement, fields, fieldName)
	}

	values := make([]string, 0)
	for _, v := range fields[fieldName] {
		if isIPField(fieldName) {
			values = append(values, formatIPCondition(v))
			continue
		}

		values = append(values, fmt.Sprintf("'%s'", v))
	}

	return fmt.Sprintf("%s IN (%s)", statement, strings.Join(values, ", "))
}

func prefixMultiValueCondition(statement string, fields url.Values, fieldName string) string {
	conditions := make([]string, 0)
	for _, v := range fields[fieldName] {
		var err error
		v, err = formatPrefixCondition(fieldName, v)
		if err != nil {
			return ""
		}
		conditions = append(conditions, v)
	}

	return "(" + strings.Join(conditions, " OR ") + ")"
}

func isIPField(fieldName string) bool {
	return fieldName == "nexthop" || fieldName == "src_ip_addr" || fieldName == "dst_ip_addr" || fieldName == "agent"
}

func isPrefixField(fieldName string) bool {
	return fieldName == "dst_ip_pfx" || fieldName == "src_ip_pfx"
}

func formatIPCondition(addr string) string {
	if strings.Contains(addr, ".") {
		return fmt.Sprintf("IPv4ToIPv6(IPv4StringToNum('%s'))", addr)
	}

	return fmt.Sprintf("IPv6StringToNum('%s')", addr)
}

func formatPrefixCondition(fieldName string, p string) (string, error) {
	pfx, err := bnet.PrefixFromString(p)
	if err != nil {
		return "", err
	}

	if pfx.Addr().IsIPv4() {
		return fmt.Sprintf("(%s_addr = IPv4ToIPv6(IPv4StringToNum('%s')) AND %s_len = %d)", fieldName, pfx.Addr().String(), fieldName, pfx.Pfxlen()), nil
	}

	return fmt.Sprintf("(%s_addr = IPv6StringToNum('%s') AND %s_len = %d)", fieldName, pfx.Addr().String(), fieldName, pfx.Pfxlen()), nil
}

func resolveVirtualField(f string) string {
	if f == "src_ip_pfx" {
		return "concat(IPv6NumToString(src_ip_pfx_addr), '/', toString(src_ip_pfx_len))"
	}

	if f == "dst_ip_pfx" {
		return "concat(IPv6NumToString(dst_ip_pfx_addr), '/', toString(dst_ip_pfx_len))"
	}

	return f
}

func timeFieldToTimestamp(v string) (int64, error) {
	v += ":00+00:00" // FIXME: Make this configurable
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0, errors.Wrapf(err, "Unable to parse %q", v)
	}

	return t.Unix(), nil
}

// resolveDictIfNecessary maps a fieldname to an dict lookup, if necessary. If not it just returns fieldname.
func (fe *Frontend) resolveDictIfNecessary(fieldName string) (string, error) {
	flowsFieldName, relatedFieldsName := parseFieldName(fieldName)
	if relatedFieldsName == "" {
		return flowsFieldName, nil
	}

	d := fe.dictCfgs.getDict(flowsFieldName)
	if d == nil {
		return "", fmt.Errorf("Dict for field %s not found", fieldName)
	}

	dictName := d.Dict
	if !strings.Contains(dictName, ".") {
		dictName = fe.chgw.GetDatabaseName() + "." + dictName
	}

	return fmt.Sprintf("dictGet('%s', '%s', %s)", dictName, relatedFieldsName, d.Expr), nil
}

func parseFieldName(name string) (flowsFieldName, relatedFieldsName string) {
	parts := strings.Split(name, "__")
	if len(parts) < 2 {
		return parts[0], ""
	}

	return parts[0], parts[1]
}

func (fe *Frontend) dissectIndexQuery(values url.Values) map[string][]string {
	fields := make(map[string][]string)
	for k, v := range values {
		if strings.HasPrefix(k, "filter_field") {
			continue
		}

		fields[k] = v
	}

	return fields
}

func (fe *Frontend) getIndexView() (*IndexView, error) {
	caser := cases.Title(language.English)

	ret := &IndexView{
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

		for _, dictCfg := range fe.dictCfgs {
			if dictCfg.Field != field.Name {
				continue
			}

			dictFields, err := fe.chgw.GetDictFields(dictCfg.Dict)
			if err != nil {
				log.Errorf("failed to get dict fields: %v", err)
				continue
			}

			for i := 0; i < len(dictFields); i++ {
				label := strings.Replace(dictFields[i], "_", " ", -1)
				f := &Field{
					Name:  fmt.Sprintf("%s__%s", field.Name, dictFields[i]),
					Label: fmt.Sprintf("%s %s", field.Label, caser.String(label)),
				}
				fg.Fields = append(fg.Fields, f)

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

	res := make([]string, 0)
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
		return "", "", fmt.Errorf("invalid format")
	}

	return parts[0], parts[1], nil
}
