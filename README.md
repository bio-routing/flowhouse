# flowhouse

Flowhouse is a Clickhouse based sFlow collector and web based analyzer that offers rich annotation and querying features.

![screenshot](https://github.com/bio-routing/flowhouse/raw/master/assets/flowhouse-ui.png "UI Screenshot")

It is planned to add support for IPFIX/Netflow version 9. Packet decoder to be taken from github.com/bio-routing/tflow2.
Patches adding support are very welcome!

## Interface Name Discovery

Discovery of interface names is supported using SNMP. The database always stores interface namens. Not IDs.

## Statis Meta Data Annotations

Static meta data annotations are supported by the use of Clickhouses dicts.

## Dynamic Routing Meta Data Annotations

Dynamic routing meta data annotations like source and destination prefix, source, destination and nexthop ASN are supported
on the basis of the BIO routing RIS.
