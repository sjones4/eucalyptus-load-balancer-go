// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	ExampleLoadBalancer = `<LoadBalancerDescriptions xmlns="http://elasticloadbalancing.amazonaws.com/doc/2012-06-01/"><member><LoadBalancerName>balancer-1</LoadBalancerName><DNSName>balancer-1-000174477311.lb.box3-10-111-10-63.euca.me</DNSName><ListenerDescriptions><member><Listener><Protocol>HTTP</Protocol><LoadBalancerPort>8080</LoadBalancerPort><InstancePort>8080</InstancePort></Listener><PolicyNames><member>sticky</member></PolicyNames></member></ListenerDescriptions><PolicyDescriptions/><AvailabilityZones><member>one</member></AvailabilityZones><HealthCheck><Target>TCP:8080</Target><Interval>30</Interval><Timeout>5</Timeout><UnhealthyThreshold>3</UnhealthyThreshold><HealthyThreshold>3</HealthyThreshold></HealthCheck><CreatedTime>2020-04-02T16:18:19.451Z</CreatedTime><LoadBalancerAttributes><CrossZoneLoadBalancing><Enabled>false</Enabled></CrossZoneLoadBalancing><AccessLog><Enabled>true</Enabled></AccessLog><ConnectionDraining><Enabled>false</Enabled></ConnectionDraining><ConnectionSettings><IdleTimeout>60</IdleTimeout></ConnectionSettings></LoadBalancerAttributes></member></LoadBalancerDescriptions>`

	// Each value is a single policy, but the message uses a list structure
	ExamplePolicy = `<LoadBalancerDescriptions xmlns="http://elasticloadbalancing.amazonaws.com/doc/2012-06-01/"><member><PolicyDescriptions><member><PolicyName>sticky</PolicyName><PolicyTypeName>LBCookieStickinessPolicyType</PolicyTypeName><PolicyAttributeDescriptions><member><AttributeName>CookieExpirationPeriod</AttributeName><AttributeValue>300</AttributeValue></member></PolicyAttributeDescriptions></member></PolicyDescriptions></member></LoadBalancerDescriptions>`
)

func TestLoadBalancerRead(t *testing.T) {
	descriptions, err := ActivityDescriptionsString(ExampleLoadBalancer)
	if err != nil {
		t.Errorf("ActivityDescriptionsString(ExampleLoadBalancer) = _, error; %s", err.Error())
	}
	t.Logf("%+v\n", descriptions)
	assert.Equal(t, 1, len(descriptions.LoadBalancers), "len(descriptions.LoadBalancers)")
	assert.Equal(t, "balancer-1", descriptions.LoadBalancers[0].LoadBalancerName,
		"descriptions.LoadBalancers[0].LoadBalancerName")
	assert.Equal(t, "balancer-1-000174477311.lb.box3-10-111-10-63.euca.me", descriptions.LoadBalancers[0].DNSName,
		"descriptions.LoadBalancers[0].DNSName")
	assert.Equal(t, 1, len(descriptions.LoadBalancers[0].Listeners), "len(descriptions.LoadBalancers[0].Listeners)")
	assert.Equal(t, ActivityLoadBalancerListener{Protocol: "HTTP", LoadBalancerPort: 8080, InstancePort: 8080, PolicyNames: []string{"sticky"}},
		descriptions.LoadBalancers[0].Listeners[0],
		"descriptions.LoadBalancers[0].Listeners[0]")
	assert.Equal(t, []string{"one"}, descriptions.LoadBalancers[0].AvailabilityZones,
		"descriptions.LoadBalancers[0].AvailabilityZones")
	assert.Equal(t, ActivityHealthCheck{Target: "TCP:8080", Interval: 30, Timeout: 5, UnhealthyThreshold: "3", HealthyThreshold: "3"},
		descriptions.LoadBalancers[0].HealthCheck,
		"descriptions.LoadBalancers[0].HealthCheck")
	assert.Equal(t, ActivityLoadBalancerAttributes{CrossZoneLoadBalancing: false, AccessLog: true, ConnectionDraining: false, ConnectionSettings: ActivityConnectionSettings{IdleTimeout: 60}},
		descriptions.LoadBalancers[0].LoadBalancerAttributes,
		"descriptions.LoadBalancers[0].LoadBalancerAttributes")
}

func TestPolicyRead(t *testing.T) {
	descriptions, err := ActivityDescriptionsString(ExamplePolicy)
	if err != nil {
		t.Errorf("ActivityDescriptionsString(ExamplePolicy) = _, error; %s", err.Error())
	}
	t.Logf("%+v\n", descriptions)
	assert.Equal(t, 1, len(descriptions.LoadBalancers), "len(descriptions.LoadBalancers)")
	assert.Equal(t, 1, len(descriptions.LoadBalancers[0].PolicyDescriptions),
		"len(descriptions.LoadBalancers[0].PolicyDescriptions)")
	assert.Equal(t, "sticky", descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyName,
		"descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyName")
	assert.Equal(t, "LBCookieStickinessPolicyType", descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyTypeName,
		"descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyTypeName")
	assert.Equal(t, 1, len(descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyAttributes),
		"len(descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyAttributes)")
	assert.Equal(t, ActivityPolicyAttribute{AttributeName: "CookieExpirationPeriod", AttributeValue: "300"},
		descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyAttributes[0],
		"descriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyAttributes[0]")
}
