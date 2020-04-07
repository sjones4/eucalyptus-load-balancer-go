// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"errors"
	"fmt"
	"github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/common"
	"github.com/haproxytech/config-parser/v2/params"
	"github.com/haproxytech/config-parser/v2/parsers/http/actions"
	"github.com/haproxytech/config-parser/v2/types"
	"io/ioutil"
	"strings"
)

var PolicyCache = &HAproxyPolicyCache{map[string]ActivityPolicy{}}

// HA-Proxy configuration
type HaproxyConfiguration struct {
	Parser *parser.Parser
}

type HAproxyPolicyCache struct {
	Policies map[string]ActivityPolicy
}

// ActivityHandler implementation for receiving configuration
type HaproxyConfigurationHandler struct {
	TemplateSupplier      func() (string, error)
	ConfigurationReceiver func(string) error
}

func HaproxyConfigurationString(configuration string) (haproxyConfiguration *HaproxyConfiguration, err error) {
	haproxyParser := parser.Parser{}
	haproxyConfiguration = &HaproxyConfiguration{&haproxyParser}
	err = haproxyParser.ParseData(configuration)
	return
}

func HaproxyConfigurationFile(filename string) (haproxyConfiguration *HaproxyConfiguration, err error) {
	haproxyParser := parser.Parser{}
	haproxyConfiguration = &HaproxyConfiguration{&haproxyParser}
	err = haproxyParser.LoadData(filename)
	return
}

// Set a default socket timeout for the configuration
// Set a "check", "connect", "client", "server", etc timeout (e.g. "15s", "1m")
// in the defaults configuration section.
func (configuration *HaproxyConfiguration) SetDefaultTimeout(timeout string, value string) (err error) {
	data, err := configuration.Parser.Get(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout))
	if err != nil {
		return
	}
	dataTimeout, ok := data.(*types.SimpleTimeout)
	if !ok {
		return errors.New(fmt.Sprintf("Invalid type for timeout: %#v", data))
	}
	dataTimeout.Value = value
	return
}

// Get the configuration as a string
func (configuration *HaproxyConfiguration) String() string {
	return configuration.Parser.String()
}

// Create an ActivityHandler that outputs HAProxy configuration
// The handler listens for loadbalancer and policy data and outputs an HAProxy
// configuration based on the given "template" and data.
func NewHaproxyConfigurationHandler(templatePath string, configurationPath string) ActivityHandler {
	templateFromFile := func() (string, error) {
		data, err := ioutil.ReadFile(templatePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	configurationToFile := func(data string) error {
		return ioutil.WriteFile(configurationPath, []byte(data), 0600)
	}
	handler := &HaproxyConfigurationHandler{
		templateFromFile,
		configurationToFile,
	}
	return handler
}

func (handler *HaproxyConfigurationHandler) Send(name string, value string) error {
	switch name {
	case "set-policy":
		return handler.HandlePolicy(value)
	case "set-loadbalancer":
		return handler.HandleLoadBalancer(value)
	}
	return nil
}

func (handler *HaproxyConfigurationHandler) Receive(_ string) (*string, error) {
	return nil, errors.New("not supported")
}

func (handler *HaproxyConfigurationHandler) Close() {
}

func (handler *HaproxyConfigurationHandler) HandlePolicy(policy string) error {
	activityDescriptions, err := ActivityDescriptionsString(policy)
	if err == nil &&
		len(activityDescriptions.LoadBalancers) == 1 &&
		len(activityDescriptions.LoadBalancers[0].PolicyDescriptions) == 1 {
		name := activityDescriptions.LoadBalancers[0].PolicyDescriptions[0].PolicyName
		activityPolicy := activityDescriptions.LoadBalancers[0].PolicyDescriptions[0]
		PolicyCache.Policies[name] = activityPolicy
	}
	return err
}

func (handler *HaproxyConfigurationHandler) HandleLoadBalancer(loadBalancer string) error {
	activityDescriptions, err := ActivityDescriptionsString(loadBalancer)
	if err == nil &&
		len(activityDescriptions.LoadBalancers) == 1 &&
		len(activityDescriptions.LoadBalancers[0].PolicyDescriptions) == 0 {
		loadBalancer := activityDescriptions.LoadBalancers[0]
		activePolicyNames := map[string]string{}
		for _, listener := range loadBalancer.Listeners {
			for _, policyName := range listener.PolicyNames {
				activePolicyNames[policyName] = policyName
			}
		}
		for _, backend := range loadBalancer.BackendServers {
			for _, policyName := range backend.PolicyNames {
				activePolicyNames[policyName] = policyName
			}
		}
		for _, policyName := range activePolicyNames {
			if activePolicy, ok := PolicyCache.Policies[policyName]; ok {
				loadBalancer.PolicyDescriptions = append(loadBalancer.PolicyDescriptions, activePolicy)
			} else {
				return errors.New(fmt.Sprintf("policy not found %s", policyName))
			}
		}
		PolicyCache.RetainOnly(activePolicyNames)
		return handler.WriteConfiguration(&loadBalancer)
	}
	return err
}

// Purge stale cached items by retaining only keys from the given map
func (cache *HAproxyPolicyCache) RetainOnly(retainKeys map[string]string) {
	var stalePolicyNames []string
	for policyName := range PolicyCache.Policies {
		if _, ok := retainKeys[policyName]; !ok {
			stalePolicyNames = append(stalePolicyNames, policyName)
		}
	}
	for _, policyName := range stalePolicyNames {
		delete(PolicyCache.Policies, policyName)
	}
}

func (handler *HaproxyConfigurationHandler) WriteConfiguration(loadBalancer *ActivityLoadBalancer) error {
	configuration, err := handler.TemplateSupplier()
	if err != nil {
		return err
	}
	haproxyConfiguration, err := HaproxyConfigurationString(configuration)
	if err != nil {
		return err
	}
	err = UpdateConfiguration(haproxyConfiguration, loadBalancer)
	if err != nil {
		return err
	}
	err = handler.ConfigurationReceiver(haproxyConfiguration.String())
	return err
}

func UpdateConfigurationSection(haproxyConfiguration *HaproxyConfiguration, sectionType parser.Section, sectionName string, attributes map[string]common.ParserData) error {
	err := haproxyConfiguration.Parser.SectionsCreate(sectionType, sectionName)
	if err != nil {
		return err
	}

	for name, value := range attributes {
		err = haproxyConfiguration.Parser.Set(sectionType, sectionName, name, value)
		if err != nil {
			return err
		}
	}

	return nil
}

// Proof of concept configuration update
// The given loadBalancer should be used to generate configuration. Currently
// values are hard-coded for testing configuration output.
func UpdateConfiguration(haproxyConfiguration *HaproxyConfiguration, loadBalancer *ActivityLoadBalancer) error {
	frontendAttributes := map[string]common.ParserData{}
	frontendAttributes["mode"] = configStringC("http")
	frontendAttributes["bind"] = &types.Bind{Path: "0.0.0.0:8080"}
	frontendAttributes["log-format"] = configStringC("httplog %Ts %ci %cp %si %sp %Tq %Tw %Tc %Tr %Tt %ST %U %B %f %b %s %ts %r %hrl")
	frontendAttributes["log"] = &types.Log{Address: "/var/lib/load-balancer-servo/haproxy.sock", Facility: "local2", Level: "info"}
	frontendAttributes["option forwardfor"] = &types.OptionForwardFor{Except: "127.0.0.1"}
	frontendAttributes["timeout client"] = &types.SimpleTimeout{Value: "60s"}
	frontendAttributes["default_backend"] = configStringC("backend-http-8080")
	frontendAttributes["http-request"] = []types.HTTPAction{
		&actions.SetHeader{Name: "X-Forwarded-Proto", Fmt: "http"},
		&actions.SetHeader{Name: "X-Forwarded-Port", Fmt: "8080"},
		//TODO syntax not supported by haproxy 1.5
		// &actions.Capture{Sample: "hdr(User-Agent)", Len: configInt64(8192)},
	}
	err := UpdateConfigurationSection(haproxyConfiguration, parser.Frontends, "http-8080", frontendAttributes)
	if err != nil {
		return err
	}

	backendAttributes := map[string]common.ParserData{}
	backendAttributes["mode"] = configStringC("http")
	backendAttributes["balance"] = &types.Balance{"roundrobin", nil, ""}
	backendAttributes["http-response"] = &actions.SetHeader{Name: "Cache-control", Fmt: `no-cache="set-cookie"`}
	backendAttributes["cookie"] = &types.Cookie{Name: "AWSELB", Type: "insert", Indirect: true, Maxidle: 300000, Maxlife: 300000}
	backendAttributes["server"] = []types.Server{{Name: "http-8080", Address: "10.111.10.215:8080", Params: []params.ServerOption{&params.ServerOptionValue{Name: "cookie", Value: "MTAuMTExLjEwLjIxNQ=="}}}}
	backendAttributes["timeout server"] = &types.SimpleTimeout{Value: "60s"}
	err = UpdateConfigurationSection(haproxyConfiguration, parser.Backends, "backend-http-8080", backendAttributes)
	return err
}

func configInt64(value int64) *int64 {
	return &value
}

func configStringC(value string) *types.StringC {
	return &types.StringC{Value: strings.ReplaceAll(value, " ", `\ `)}
}
