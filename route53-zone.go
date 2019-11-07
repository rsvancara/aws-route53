package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"gopkg.in/yaml.v2"
)

// Configuration file path used in command line parameters
var config string

// Represents a route53Zone configuration
type route53Zone struct {
	Name               string              `yaml:"Name"`
	ZoneID             string              `yaml:"ZoneID"`
	ResourceRecordSets []resourceRecordSet `yaml:"ResourceRecordSets"`
}

// Represents a route53 aliasTarget configuration
type aliasTarget struct {
	HostedZoneID         string `yaml:"HostedZoneID"`
	DNSName              string `yaml:"DNSName"`
	EvaluateTargetHealth bool   `yaml:"EvaluateTargetHealth"`
}

//  Function Receiver for TargetHostedZone
func (i *aliasTarget) getAliasTargetHostedZoneID() string {
	if i == nil {
		return ""
	}
	return i.HostedZoneID
}

// Function Receiver for AliasDNSName
func (i *aliasTarget) getAliasDNSName() string {
	if i == nil {
		return ""
	}
	return i.DNSName
}

// Represents resource record configuration
type resourceRecords struct {
	Value string `yaml:"Value"`
}

// Represents resource recordset configuration
type resourceRecordSet struct {
	TTL             int64             `yaml:"TTL"`
	Name            string            `yaml:"Name"`
	Type            string            `yaml:"Type"`
	AliasTarget     aliasTarget       `yaml:"AliasTarget,omitempty"`
	ResourceRecords []resourceRecords `yaml:"ResourceRecords,omitempty"`
}

// Initializing command line
func init() {
	flag.StringVar(&config, "c", "", "configuration")
}

// Main function
func main() {

	flag.Parse()

	if config == "" {
		fmt.Println(fmt.Errorf("incomplete arguments: c: %s", config))
		flag.PrintDefaults()
		return
	}

	if fileExists(config) != true {
		fmt.Println(fmt.Errorf("configuration file %s does not exist", config))
		flag.PrintDefaults()
		return
	}

	zoneConfig, err := readConfig(config)
	if err != nil {
		log.Fatal("error")
	}

	// One way to create a session...
	//sess, err := session.NewSession(&aws.Config{
	//	Region: aws.String("us-west-2")})

	// A little better way to create a session
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		log.Fatalf("failed to create session, %s", err)
	}

	svc := route53.New(sess)

	deltaBuilder(svc, zoneConfig)
}

// Print the formatted summary to display at the end of the command
// execution for summary purposes.  Describes what changed.
func printReport(changes []*route53.Change) {

	fmt.Println("Propsed Changes:")
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', tabwriter.Debug|tabwriter.AlignRight)
	fmt.Fprintln(w, "ACTION\tNAME\tTYPE")

	for _, change := range changes {
		fmt.Fprintln(w, fmt.Sprintf("%s\t%s\t%s", aws.StringValue(change.Action),
			aws.StringValue(change.ResourceRecordSet.Name),
			aws.StringValue(change.ResourceRecordSet.Type)))
	}
	w.Flush()
}

// Read the provided YAML formated configuration file into
// a route53Zone datatype.
func readConfig(config string) (*route53Zone, error) {

	r := route53Zone{}

	content, err := ioutil.ReadFile(config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	err = yaml.Unmarshal(content, &r)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	return &r, nil
}

// Takes an array of route53.Change types and submits it to AWS.
// Returns an error if there is a failure
func createResourceRecordSetChange(svc *route53.Route53, zone string, changes []*route53.Change) error {
	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{ // Required
			Changes: changes,
			Comment: aws.String("Zone Changes"),
		},
		HostedZoneId: aws.String(zone), // Required
	}
	resp, err := svc.ChangeResourceRecordSets(params)
	if err != nil {
		return err
	}

	// Pretty-print the response data.
	fmt.Println("Changes Submitted to AWS:")
	fmt.Printf("Comment:     %s \n", aws.StringValue(resp.ChangeInfo.Comment))
	fmt.Printf("ID:          %s \n", aws.StringValue(resp.ChangeInfo.Id))
	fmt.Printf("Status:      %s \n", aws.StringValue(resp.ChangeInfo.Status))
	fmt.Printf("Submitted At: %s \n", aws.TimeValue(resp.ChangeInfo.SubmittedAt))
	return nil
}

// Test if a file exists, used to validate configuration file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// Find resourcerecords that can be deleted.  These are the records that are NOT
// in the configuration but do exist in the route53 zone
func findRecordsToDelete(configrr *route53Zone, awsrr []*route53.ResourceRecordSet) []*route53.Change {

	var diff []*route53.Change
	len1 := len(awsrr)
	len2 := len(configrr.ResourceRecordSets)

	for i := 1; i < len1; i++ {
		var j int
		for j = 0; j < len2; j++ {
			// Ignore NS records, please do not delete these
			if aws.StringValue(awsrr[i].Type) == "NS" || aws.StringValue(awsrr[i].Type) == "SOA" {
				break
			}
			// Find a match, short circuit and go to the next iteration
			if configrr.ResourceRecordSets[j].Name == aws.StringValue(awsrr[i].Name) &&
				configrr.ResourceRecordSets[j].Type == aws.StringValue(awsrr[i].Type) {
				break
			}
		}
		if j == len2 {
			diff = append(diff, &route53.Change{Action: aws.String("DELETE"), ResourceRecordSet: awsrr[i]})
		}
	}

	return diff
}

// Find records that can be added.  These are records that are in the Cconfiguration
// but not in the route53 zone
func findRecordsToAdd(configrr *route53Zone, awsrr []*route53.ResourceRecordSet) []*route53.Change {

	var diff []*route53.Change
	len1 := len(configrr.ResourceRecordSets)
	len2 := len(awsrr)

	for i := 1; i < len1; i++ {
		var j int
		for j = 0; j < len2; j++ {
			// Find a match, short circuit and go to the next iteration
			if configrr.ResourceRecordSets[i].Name == aws.StringValue(awsrr[j].Name) &&
				configrr.ResourceRecordSets[i].Type == aws.StringValue(awsrr[j].Type) {
				break
			}
		}
		if j == len2 {
			change, err := getChange("CREATE", &configrr.ResourceRecordSets[i])
			if err != nil {
				log.Fatalf("Error getting change will adding recordset %s with error: %s ",
					configrr.ResourceRecordSets[i].Name, err)
			}
			diff = append(diff, change)
		}
	}

	return diff
}

// Generate the route53.Change object from the config
func getChange(changeType string, configrr *resourceRecordSet) (*route53.Change, error) {

	var changeRR []*route53.ResourceRecord

	if configrr.ResourceRecords != nil {
		for _, trr := range configrr.ResourceRecords {
			value := trr.Value
			changeRR = append(changeRR, &route53.ResourceRecord{Value: &value})
		}
		var change = route53.Change{
			Action: aws.String(changeType), // Required
			ResourceRecordSet: &route53.ResourceRecordSet{ // Required
				Name:            aws.String(configrr.Name), // Required
				Type:            aws.String(configrr.Type), // Required
				TTL:             aws.Int64(300),
				ResourceRecords: changeRR,
			},
		}

		return &change, nil
	}

	if configrr.AliasTarget.getAliasDNSName() != "" {
		//var at route53.AliasTarget
		at := route53.AliasTarget{
			DNSName:              aws.String(configrr.AliasTarget.DNSName),
			HostedZoneId:         aws.String(configrr.AliasTarget.HostedZoneID),
			EvaluateTargetHealth: aws.Bool(configrr.AliasTarget.EvaluateTargetHealth),
		}
		var change = route53.Change{
			Action: aws.String(changeType), // Required
			ResourceRecordSet: &route53.ResourceRecordSet{ // Required
				Name: aws.String(configrr.Name), // Required
				Type: aws.String(configrr.Type), // Required
				//TTL:         aws.Int64(300),
				AliasTarget: &at,
			},
		}
		return &change, nil
	}

	return nil, fmt.Errorf(fmt.Sprintf("no value was changed for record %s", configrr.Name))
}

// deltaBuilder constructs a resource record changeset based on the differences between the
// provided configuration and the
func deltaBuilder(svc *route53.Route53, config *route53Zone) {

	var changes []*route53.Change

	// Obtain the current records for the zone in the provided configuration
	records, err := listAllRecordSets(svc, config.ZoneID)
	if err != nil {
		log.Fatalf("Error obtaining records for zone %s with error %s", config.Name, err)
	}

	for _, crr := range config.ResourceRecordSets {
		found := false
		for _, rr := range records {
			if crr.Name == aws.StringValue(rr.Name) && crr.Type == aws.StringValue(rr.Type) {
				found = true
				break
			}
		}
		if found == true {
			exists := false
			for _, change := range changes {
				if aws.StringValue(change.ResourceRecordSet.Name) == crr.Name && aws.StringValue(change.ResourceRecordSet.Type) == crr.Type {
					exists = true
					break
				}
			}
			if exists == false {
				c, err := getChange("UPSERT", &crr)
				if err != nil {
					log.Fatalf("Error getting change to %s with error %s", crr.Name, err)
				}
				changes = append(changes, c)
			}
		}
	}

	deletediff := findRecordsToDelete(config, records)
	changes = append(changes, deletediff...)

	creatediff := findRecordsToAdd(config, records)
	changes = append(changes, creatediff...)
	printReport(changes)

	err = createResourceRecordSetChange(svc, config.ZoneID, changes)
	if err != nil {
		log.Fatalf("Error create resource record change with error: %s", err)
	}
}

// Find all the hosted zones in an AWS account
// It returns a map of all the hosted zones
func getHostedZones(svc *route53.Route53) (map[string]*route53.HostedZone, error) {

	zones := make(map[string]*route53.HostedZone)

	f := func(resp *route53.ListHostedZonesOutput, lastPage bool) (shouldContinue bool) {
		for _, zone := range resp.HostedZones {
			zones[*zone.Id] = zone
		}
		return true
	}

	err := svc.ListHostedZonesPages(&route53.ListHostedZonesInput{}, f)
	if err != nil {
		return nil, err
	}

	return zones, nil
}

// Obtains the RecordSets for a provided zone.
// Returns a *route53.ListResourceRecordSetsOutput
func getHostedZoneRecords(svc *route53.Route53, zone *string) (*route53.ListResourceRecordSetsOutput, error) {

	rrInput := &route53.ListResourceRecordSetsInput{
		HostedZoneId: zone,
	}
	hostedZoneRecordSets, err := svc.ListResourceRecordSets(rrInput)

	if err != nil {
		fmt.Printf("error obtaining hosted zone %s by id:  %s", aws.StringValue(zone), err)
		return nil, err
	}

	return hostedZoneRecordSets, nil
}

// Paginate request to get all record sets.
func listAllRecordSets(r53 *route53.Route53, id string) (rrsets []*route53.ResourceRecordSet, err error) {
	req := route53.ListResourceRecordSetsInput{
		HostedZoneId: &id,
	}

	for {
		var resp *route53.ListResourceRecordSetsOutput
		resp, err = r53.ListResourceRecordSets(&req)
		if err != nil {
			return
		}
		rrsets = append(rrsets, resp.ResourceRecordSets...)
		if *resp.IsTruncated {
			req.StartRecordName = resp.NextRecordName
			req.StartRecordType = resp.NextRecordType
			req.StartRecordIdentifier = resp.NextRecordIdentifier
		} else {
			break
		}
	}

	// unescape wildcards
	//for _, rrset := range rrsets {
	//	rrset.Name = aws.String(unescaper.Replace(*rrset.Name))
	//}

	return
}

// Look up a hosted zone by ID
func getHostedZoneIDByNameLookup(svc *route53.Route53, hostedZoneName string) (string, error) {

	listParams := &route53.ListHostedZonesByNameInput{
		DNSName:  aws.String(hostedZoneName), // Required
		MaxItems: aws.String("1"),
	}
	hzOut, err := svc.ListHostedZonesByName(listParams)
	if err != nil {
		return "", err
	}

	zones := hzOut.HostedZones

	if len(zones) < 1 {
		fmt.Printf("No zone found for %s\n", hostedZoneName)
		return "", err
	}

	zoneID := *zones[0].Id

	// remove the /hostedzone/ path if it's there
	if strings.HasPrefix(zoneID, "/hostedzone/") {
		zoneID = strings.TrimPrefix(zoneID, "/hostedzone/")
	}

	return zoneID, nil
}
