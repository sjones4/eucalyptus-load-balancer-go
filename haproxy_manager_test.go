// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"testing"
)

const TemplateConf = `#template
global
 maxconn 100000
 ulimit-n 655360
 pidfile /var/run/haproxy.pid

#drop privileges after port binding
 user servo
 group servo

defaults
 timeout connect     5s
 timeout client      1m
 timeout server      1m
 errorfile 503 /etc/load-balancer-servo/503.http`

const ExampleConf = `global
  maxconn 100000
  ulimit-n 655360
  pidfile /var/run/haproxy.pid
  #drop privileges after port binding
  user servo
  group servo

defaults
  timeout connect     5
  timeout client      60
  timeout server      60
  errorfile 503 /etc/load-balancer-servo/503.http

frontend http-8080
  # lb-balancer-1
  mode http
  option forwardfor except 127.0.0.1
  reqadd X-Forwarded-Proto:\ http
  reqadd X-Forwarded-Port:\ 8080
  bind 10.111.10.233:8080
  timeout client 60s
  log /var/lib/load-balancer-servo/haproxy.sock local2 info
  capture request header User-Agent len 8192
  log-format httplog\ %Ts\ %ci\ %cp\ %si\ %sp\ %Tq\ %Tw\ %Tc\ %Tr\ %Tt\ %ST\ %U\ %B\ %f\ %b\ %s\ %ts\ %r\ %hrl
  default_backend backend-http-8080

backend backend-http-8080
  mode http
  balance roundrobin
  timeout server 60s
  http-response set-header Cache-control no-cache="set-cookie"
  cookie AWSELB insert indirect maxidle 300000 maxlife 300000
  server http-8080 10.111.10.215:8080 cookie MTAuMTExLjEwLjIxNQ==
`

func TestHaproxyTemplateConf(t *testing.T) {
	configuration, err := HaproxyConfigurationString(TemplateConf)
	if err != nil {
		t.Fatalf("HaproxyConfigurationString(TemplateConf) error; %s", err.Error())
	}
	t.Log(configuration.Parser.String())
}

func TestHaproxyConf(t *testing.T) {
	configuration, err := HaproxyConfigurationString(ExampleConf)
	if err != nil {
		t.Fatalf("HaproxyConfigurationString(ExampleConf) error; %s", err.Error())
	}
	t.Log(configuration.Parser.String())
	err = configuration.SetDefaultTimeout("connect", "15s")
	if err != nil {
		t.Fatalf("configuration.SetDefaultTimeout(connect, 15s) error; %s", err.Error())
	}
	t.Log(configuration.Parser.String())
}

func TestHaproxyConfigurationHandler(t *testing.T) {
	templateStatic := func() (string, error) {
		return TemplateConf, nil
	}
	configurationLogger := func(data string) error {
		t.Log(data)
		return nil
	}
	handler := &HaproxyConfigurationHandler{
		templateStatic,
		configurationLogger}
	err := handler.Send("set-policy", ExamplePolicy)
	if err != nil {
		t.Fatal(err.Error())
	}
	err = handler.Send("set-loadbalancer", ExampleLoadBalancer)
	if err != nil {
		t.Fatal(err.Error())
	}
}
