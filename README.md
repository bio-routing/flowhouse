# flowhouse

Flowhouse is a [Clickhouse](https://clickhouse.tech/) based sFlow collector and web based analyzer that offers rich annotation and querying features.

![screenshot](https://github.com/bio-routing/flowhouse/raw/master/assets/flowhouse-ui.png "UI Screenshot")

It is planned to add support for IPFIX/Netflow version 9. Packet decoder to be taken from [github.com/bio-routing/tflow2](https://github.com/bio-routing/tflow2).
Patches adding support are very welcome!

## Interface Name Discovery

Discovery of interface names is supported using SNMP. The database always stores interface namens. Not IDs.

## Static Meta Data Annotations

Static meta data annotations are supported by the use of Clickhouses dicts.

## Dynamic Routing Meta Data Annotations

Dynamic routing meta data annotations like source and destination prefix, source, destination and nexthop ASN are supported
on the basis of the [BIO routing RIS](https://github.com/bio-routing/bio-rd/tree/master/cmd/ris).

## Installation
```go get github.com/bio-routing/flowhouse/cmd/flowhouse```

```go install github.com/bio-routing/flowhouse/cmd/flowhouse```

## Configuration

We have not documentation about this yet. But the config format is defined here: [https://github.com/bio-routing/flowhouse/blob/master/cmd/flowhouse/config/config.go#L21](https://github.com/bio-routing/flowhouse/blob/master/cmd/flowhouse/config/config.go#L21)

## Running it
```user@host ~ % flowhouse --help
Usage of flowhouse:
  -config.file string
    	Config file path (YAML) (default "config.yaml")
```
