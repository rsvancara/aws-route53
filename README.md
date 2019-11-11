# AWS-ROUTE53

This tool will take a YAML file and synchronize the YAML configuration to an AWS Route53 
hosted zone.  It assumes the configuration file is authoritative and remove records
that are not in the configuration file, so be careful while testing.

Additionally this tool will extract all DNS configurations from an AWS account.  See usage below.

## Usage

### Synchronize a configuration to route53

```bash
route53-zone -c path_to_configuration file
```

### Extract all route53 configurations from an AWS account

```bash
route53-zone -p somedirectory/you/want/to/use -b -a 
```

## Configuration file

Example configuration file:

```YAML
Name: somedomain.es.
ZoneID: Z2TW4NHZZ8XXXX
ResourceRecordSets:
    - Name: somedomain.es.
      TTL: 600
      Type: A
      ResourceRecords:
        - Value: 10.153.23.22
    - Name: fakedns.somedomain.es.
      Type: A
      TTL: 900
      ResourceRecords:
        - Value: 12.23.12.123
    - Name: fakename.somedomain.es.
      Type: A
      AliasTarget: 
        HostedZoneID: Z1H1FL5HAXXXX
        DNSName: dualstack.xxxxx-1962099819.us-west-2.elb.amazonaws.com.
        EvaluateTargetHealth: false
    - Name: farscape.somedomain.es.
      Type: CNAME
      TTL: 300
      ResourceRecords: 
        - Value: www.google.com
    - Name: anothername.somedomain.es.
      Type: CNAME
      AliasTarget: 
        HostedZoneID: Z1H1FL5HAXXXX
        DNSName: dualstack.xxxxx-1962099819.us-west-2.elb.amazonaws.com.
        EvaluateTargetHealth: false
    - Name: mx.somedomain.es.
      Type: MX
      TTL: 300
      ResourceRecords: 
        - Value: 10 mx1.google.com
        - Value: 20 mx2.google.com
        - Value: 30 mx3.google.com
        - Value: 40 mx3.google.com
    - Name: farscapev2.somedomain.es.
      Type: CNAME
      TTL: 300
      ResourceRecords: 
        - Value: www.google.com
    - Name: somedomain.es.
      Type: TXT
      TTL: 300
      ResourceRecords: 
        - Value: '"v=spf1 ip4:34.243.61.237 ip6:2a05:d018:e3:8c00:bb71:dea8:8b83:851e include:thirdpartydomain.com -all"'
        - Value: '"v=spf1 ip4:34.243.61.238 ip6:2a05:d018:e3:8c00:bb71:dea8:8b83:851e include:thirdpartydomain.com -all"'
    - Name: farscapev3.somedomain.es.
      Type: A
      TTL: 300
      ResourceRecords: 
        - Value: 10.152.45.7

```

# Additional Notes:
Not all AWS features have been implemented.  
1.  Missing configuration for GEOIP
2.  MIssing configuration for health checks for AliasTargets
3.  Probably other features missing
4.  It will not update SOA or NS records, these are filtered out by default
5.  This is considered alpha quality code.  

