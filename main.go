// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"crypto/sha1"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	// EucalyptusRegion is used for the configured endpoint
	EucalyptusRegion = "eucalyptus"

	// Cache time for activity values, from last access
	ActivityCacheSeconds = 300
)

// ActivityHandler handles in/out values for workflow activities.
// The handler is closed after each activity so will see at most
// one send and one receive.
type ActivityHandler interface {
	Send(name string, value string) error

	Receive(name string) (*string, error)

	Close()
}

// CachedValue is an activity value and time of last access
type CachedValue struct {
	Time  time.Time
	Value string
}

var (
	// Logger for the application
	logger *log.Logger

	// ActivityChannels maps workflow activity names to handler identifiers
	ActivityChannels = map[string]string{
		"LoadBalancingVmActivities.getCloudWatchMetrics": "get-cloudwatch-metrics",
		"LoadBalancingVmActivities.getInstanceStatus":    "get-instance-status",
		"LoadBalancingVmActivities.setLoadBalancer":      "set-loadbalancer",
		"LoadBalancingVmActivities.setPolicy":            "set-policy",
	}

	// ActivityDefaultValues maps workflow activity names to default values.
	// A workflow activity that has no value is an out only activity and the
	// default value will be sent.
	ActivityDefaultValues = map[string]string{
		"LoadBalancingVmActivities.getCloudWatchMetrics": "GetCloudWatchMetrics",
		"LoadBalancingVmActivities.getInstanceStatus":    "GetInstanceStatus",
	}

	// ActivityLastValues tracks the last sent value by workflow activity name.
	// BUG(s): There can be multiple policies so tracking the last one is odd
	ActivityLastValues = map[string]string{
		"LoadBalancingVmActivities.setLoadBalancer": "",
		"LoadBalancingVmActivities.setPolicy":       "",
	}

	// ActivityValuesBySha1 maps values by SHA-1 key by workflow activity name.
	// Keys are the hex string for the values UTF-8 SHA-1 hash.
	ActivityValuesBySha1 = map[string]map[string]CachedValue{
		"LoadBalancingVmActivities.setLoadBalancer": {},
		"LoadBalancingVmActivities.setPolicy":       {},
	}
)

// Command line interface options
var (
	endpoint = flag.String("e", "", "SWF Service Endpoint")
	domain   = flag.String("d", "", "SWF Domain")
	tasklist = flag.String("l", "", "SWF task list")

	_ = flag.Int("o", 30, "SWF client connection timeout")
	_ = flag.Int("m", 1, "SWF client max connections")
	_ = flag.Int("r", 1, "SWF domain retention period in days")
	_ = flag.Int("t", 1, "Polling threads count (ignored)")

	runDir = flag.String("R", "/var/run/load-balancer-servo", "Directory containing runtime files")
	logDir = flag.String("L", "/var/log/load-balancer-servo", "Directory containing log files")
)

func main() {
	flag.Parse()

	configEndpoint := endpoint
	if *configEndpoint == "" {
		*configEndpoint = "http://simpleworkflow.internal:8773"
	} else if strings.HasSuffix(*configEndpoint, "/") {
		*configEndpoint = strings.TrimSuffix(*configEndpoint, "/")
	}

	configDomain := domain
	if *configDomain == "" {
		*configDomain = "LoadbalancingDomain"
	}

	configTaskList := tasklist
	if *configTaskList == "" {
		*configTaskList = "i-00000000"
	}

	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	logFile, err := os.Create(fmt.Sprintf("%s/load-balancer-workflow.log", *logDir))
	if err == nil {
		logger.SetOutput(logFile)
	} else {
		logger.Printf("Log file error %s\n", err.Error())
	}

	logger.Printf("Using domain:%s task-list:%s endpoint:%s\n", *configDomain, *configTaskList, *configEndpoint)

	client, err := NewSwfClient(*configEndpoint, EucalyptusRegion)
	if err != nil {
		logger.Fatalf("Error creating client %s\n", err.Error())
	}

	err = client.RegisterActivities(configDomain)
	if err != nil {
		logger.Fatalf("Error registering activities %s\n", err.Error())
	}

	pollActivityTasks(client, configDomain, configTaskList)
}

// The string value or "<<none>>" if nil
func value(text *string) string {
	if text == nil {
		return "<<none>>"
	} else {
		return *text
	}
}

// Task polling loop for activity handling.
// Polls for tasks and handles as they are available using swf long polling.
//
// Polling can time out without a task being available, in which case the
// token will be nil.
func pollActivityTasks(client SwfActivityClient, domain *string, taskList *string) {
	logger.Println("Polling for tasks")
	for {
		activityTask, err := client.PollTasks(domain, taskList)
		if err == nil {
			if activityTask.Token != nil {
				taskToken := activityTask.Token
				taskActivity := activityTask.Name
				taskParam := activityTask.Parameter
				logger.Printf("Handling activity task %s parameter %s\n", *taskActivity, value(taskParam))
				activityResult, err := doActivity(*taskActivity, taskParam)
				if err == nil {
					logger.Printf("Handled activity task %s with result %s\n", *taskActivity, value(activityResult))
					err = client.RespondTaskComplete(*taskToken, activityResult)
					if err != nil {
						logger.Printf("Error responding activity task completed %s\n", err.Error())
					}
				}
				if err != nil {
					logger.Printf("Responding activity task failed %s\n", err.Error())
					failureMessage := err.Error()
					err = client.RespondTaskFailed(*taskToken, failureMessage)
					if err != nil {
						logger.Printf("Error responding activity task failed %s\n", err.Error())
					}
				}
			} else {
				logger.Println("Polling for tasks")
			}
		}
	}
}

// Handle an activity task with optional parameter
// Responsible for managing the activity value cache and handler lifecycle.
func doActivity(activity string, parameter *string) (*string, error) {
	var value string
	if parameter != nil {
		value = *parameter
	} else {
		value = ActivityDefaultValues[activity]
	}
	if lastValue, ok := ActivityLastValues[activity]; ok {
		value = activityValueCache(activity, value)
		if value != lastValue {
			ActivityLastValues[activity] = value
			storeActivityValue(ActivityChannels[activity][4:], value)
		}
	}

	handler, err := NewRedisHandler()
	if err != nil {
		logger.Printf("Error creating handler %s\n", err.Error())
		return nil, err
	}
	defer handler.Close()

	err = handler.Send(ActivityChannels[activity], value)
	if err != nil {
		logger.Printf("Error sending to handler %s\n", err.Error())
		return nil, err
	}
	if parameter == nil {
		result, _ := handler.Receive(ActivityChannels[activity])
		if err != nil {
			logger.Printf("Error receiving from handler %s\n", err.Error())
			return nil, err
		}
		logger.Printf("Response from handler %s\n", *result)
		return result, nil
	}
	return nil, nil
}

// Store an activity value to disk by name.
// Assumes all activity values are XML
func storeActivityValue(name string, value string) {
	activityValueOut, err := os.Create(fmt.Sprintf("%s/%s.xml", *runDir, name))
	if err == nil {
		defer activityValueOut.Close()
		_, err = activityValueOut.WriteString(value)
		if err != nil {
			logger.Printf("Error writing value file %s\n", err.Error())
		}
	}
}

// Handle cache for an activity value.
// The value may be a full activity value or its SHA-1 hash
func activityValueCache(activity string, value string) string {
	valueCache := ActivityValuesBySha1[activity]
	valueSha1 := value
	if match, err := regexp.MatchString("[0-9a-fA-F]{40}", value); err == nil && match {
		cachedValue, ok := valueCache[value]
		if ok {
			logger.Printf("Using cached value for %s\n", activity)
			value = cachedValue.Value
		} else {
			value = ""
		}
	} else {
		logger.Printf("Caching value for %s\n", activity)
		valueSha1 = fmt.Sprintf("%x", sha1.Sum([]byte(value)))
	}
	timeNow := time.Now()
	if value != "" {
		valueCache[valueSha1] = CachedValue{timeNow, value}
	}
	cacheMaintain(activity, timeNow)
	return value
}

// Maintain the cache by removing stale keys
func cacheMaintain(activity string, timeNow time.Time) {
	valueCache := ActivityValuesBySha1[activity]
	staleKeys := make(map[string]bool)
	for key, cachedValue := range valueCache {
		if timeNow.Second() > (cachedValue.Time.Second() + ActivityCacheSeconds) {
			staleKeys[key] = true
		}
	}
	for staleKey := range staleKeys {
		logger.Printf("Removing stale key for %s %s\n", activity, staleKey)
		delete(valueCache, staleKey)
	}
}
