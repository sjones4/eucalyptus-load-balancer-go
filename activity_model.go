// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"encoding/xml"
	"time"
)

const ActivityTimestampLayout = "2006-01-02T15:04:05.999Z"

// ActivityDescriptions is the holder for all activity values
// An activity value is a single ActivityLoadBalancer with either a single
// ActivityPolicy or all other ActivityLoadBalancer attributes
type ActivityDescriptions struct {
	LoadBalancers []ActivityLoadBalancer `xml:"member"`
}

type ActivityLoadBalancer struct {
	LoadBalancerName       string
	DNSName                string
	Scheme                 string
	VPCId                  string
	Subnets                []string                       `xml:"Subnets>member"`
	AvailabilityZones      []string                       `xml:"AvailabilityZones>member"`
	Listeners              []ActivityLoadBalancerListener `xml:"ListenerDescriptions>member"`
	PolicyDescriptions     []ActivityPolicy               `xml:"PolicyDescriptions>member"`
	BackendServers         []ActivityBackendServer        `xml:"BackendServerDescriptions>member"`
	BackendInstances       []ActivityBackendInstance      `xml:"BackendInstances>member"`
	SecurityGroups         []string                       `xml:"SecurityGroups>member"`
	SourceSecurityGroup    string
	HealthCheck            ActivityHealthCheck
	CreatedTime            ActivityTimestamp
	LoadBalancerAttributes ActivityLoadBalancerAttributes
}

type ActivityLoadBalancerListener struct {
	Protocol         string   `xml:"Listener>Protocol"`
	LoadBalancerPort int32    `xml:"Listener>LoadBalancerPort"`
	InstanceProtocol string   `xml:"Listener>InstanceProtocol"`
	InstancePort     int32    `xml:"Listener>InstancePort"`
	PolicyNames      []string `xml:"PolicyNames>member"`
}

type ActivityBackendServer struct {
	InstancePort int32
	PolicyNames  []string `xml:"PolicyNames>member"`
}

type ActivityBackendInstance struct {
	InstanceId        string
	InstanceIpAddress string
	ReportHealthCheck bool
}

type ActivityHealthCheck struct {
	Target             string
	Interval           int32
	Timeout            int32
	UnhealthyThreshold string
	HealthyThreshold   string
}

type ActivityLoadBalancerAttributes struct {
	CrossZoneLoadBalancing bool `xml:"CrossZoneLoadBalancing>Enabled"`
	AccessLog              bool `xml:"AccessLog>Enabled"`
	ConnectionDraining     bool `xml:"ConnectionDraining>Enabled"`
	ConnectionSettings     ActivityConnectionSettings
}

type ActivityConnectionSettings struct {
	IdleTimeout uint32
}

type ActivityPolicy struct {
	PolicyName       string
	PolicyTypeName   string
	PolicyAttributes []ActivityPolicyAttribute `xml:"PolicyAttributeDescriptions>member"`
}

type ActivityPolicyAttribute struct {
	AttributeName  string
	AttributeValue string
}

type ActivityTimestamp time.Time

// Parse XML descriptions string to ActivityDescriptions
func ActivityDescriptionsString(descriptions string) (activityDescriptions *ActivityDescriptions, err error) {
	activityDescriptions = &ActivityDescriptions{}
	err = xml.Unmarshal([]byte(descriptions), activityDescriptions)
	return
}

func (timestamp ActivityTimestamp) String() string {
	return time.Time(timestamp).Format(ActivityTimestampLayout)
}

func (timestamp *ActivityTimestamp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	var text string
	err = d.DecodeElement(&text, &start)
	if err != nil {
		return
	}
	parse, err := time.Parse(ActivityTimestampLayout, text)
	if err != nil {
		return
	}
	*timestamp = ActivityTimestamp(parse)
	return nil
}
