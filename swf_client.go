// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/swf"
)

const (
	// Java exception is part of the workflow activity task API
	ExceptionClass = "com.eucalyptus.loadbalancing.workflow.LoadBalancingActivityException"

	// Message key for error responses
	ExceptionMessage = "message"

	// The version to use for activity task registrations
	ActivityVersion = "1.0"

	// The default heartbeat timeout for activity task registrations
	DefaultTaskHeartbeatTimeout = "NONE"

	// The default start to close timeout for activity task registrations
	DefaultTaskStartToCloseTimeout = "60"

	// The default schedule to start timeout for activity task registrations
	DefaultTaskScheduleToStartTimeout = "60"

	// The default schedule to close timeout for activity task registrations
	DefaultTaskScheduleToCloseTimeout = "120"
)

// Result type for activity task polling
type SwfActivityTask struct {
	Token     *string
	Name      *string
	Parameter *string
}

// Facade for simple activity registration and task handling
type SwfActivityClient interface {

	// Register the pre-defined activities under the specified workflow domain
	RegisterActivities(domain *string) error

	// Poll for an activity task
	PollTasks(domain *string, taskList *string) (*SwfActivityTask, error)

	// Respond for a completed activity task
	RespondTaskComplete(token string, result *string) error

	// Respond for a failed activity task
	RespondTaskFailed(token string, message string) error
}

// Implementation of SwfActivityClient with AWS SDK client
type SwfClient struct {
	Client *swf.SWF
}

// Create a client for the given endpoint and region.
// The client will use the default credentials locations.
func NewSwfClient(endpoint string, region string) (SwfActivityClient, error) {
	sess, err := session.NewSession(&aws.Config{
		Endpoint: aws.String(endpoint),
		Region:   aws.String(region)},
	)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error creating aws session %s", err.Error()))
	}

	_, err = sess.Config.Credentials.Get()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting credentials %s", err.Error()))
	}
	var swfClient SwfActivityClient = &SwfClient{swf.New(sess)}
	return swfClient, nil
}

func (swfClient *SwfClient) RegisterActivities(domain *string) error {
	for activityName := range ActivityChannels {
		input := &swf.RegisterActivityTypeInput{
			Domain:                            domain,
			Name:                              aws.String(activityName),
			Version:                           aws.String(ActivityVersion),
			Description:                       aws.String(""),
			DefaultTaskHeartbeatTimeout:       aws.String(DefaultTaskHeartbeatTimeout),
			DefaultTaskStartToCloseTimeout:    aws.String(DefaultTaskStartToCloseTimeout),
			DefaultTaskScheduleToStartTimeout: aws.String(DefaultTaskScheduleToStartTimeout),
			DefaultTaskScheduleToCloseTimeout: aws.String(DefaultTaskScheduleToCloseTimeout),
		}
		_, err := swfClient.Client.RegisterActivityType(input)
		if err != nil {
			if svcErr, ok := err.(awserr.Error); ok {
				switch svcErr.Code() {
				case swf.ErrCodeTypeAlreadyExistsFault:
					logger.Printf("Activity type already exists %s %s\n", activityName, ActivityVersion)
				default:
					return errors.New(fmt.Sprintf("Error registering activity type %s %s: %s",
						activityName, ActivityVersion, svcErr.Error()))
				}
			} else {
				return errors.New(fmt.Sprintf("Error registering activity type %s %s: %s\n",
					activityName, ActivityVersion, err.Error()))
			}
		} else {
			logger.Printf("Registered activity type %s %s\n", activityName, ActivityVersion)
		}
	}
	return nil
}

func (swfClient *SwfClient) PollTasks(domain *string, taskList *string) (*SwfActivityTask, error) {
	input := &swf.PollForActivityTaskInput{
		Domain: domain,
		TaskList: &swf.TaskList{
			Name: taskList,
		},
		Identity: aws.String(fmt.Sprintf("client-worker-%s", *taskList)),
	}
	output, err := swfClient.Client.PollForActivityTask(input)
	if err != nil {
		return &SwfActivityTask{}, err
	}
	if output.TaskToken != nil {
		taskToken := output.TaskToken
		taskActivity := output.ActivityType.Name
		var taskParams []interface{}
		err := json.Unmarshal([]byte(*output.Input), &taskParams)
		if err != nil {
			return &SwfActivityTask{}, err
		}
		paramsArray, ok := taskParams[1].([]interface{})
		var parameter *string = nil
		if ok && len(paramsArray) > 0 {
			parameterStr := fmt.Sprintf("%s", paramsArray[0])
			parameter = &parameterStr
		}
		return &SwfActivityTask{taskToken, taskActivity, parameter}, nil
	}
	return &SwfActivityTask{}, nil
}

func (swfClient *SwfClient) RespondTaskComplete(token string, response *string) (err error) {
	responseJson, err := json.Marshal(response)
	if err != nil {
		logger.Printf("Error marshalling response %s\n", err.Error())
		return err
	} else {
		_, err = swfClient.Client.RespondActivityTaskCompleted(&swf.RespondActivityTaskCompletedInput{
			TaskToken: &token,
			Result:    aws.String(string(responseJson)),
		})
	}
	return
}

func (swfClient *SwfClient) RespondTaskFailed(token string, message string) (err error) {
	failureList := [...]interface{}{ExceptionClass, map[string]string{ExceptionMessage: message}}
	failureJson, err := json.Marshal(failureList)
	if err != nil {
		logger.Printf("Error marshalling failure result %s\n", err.Error())
		failureJson = []byte("'Unknown error'")
	}
	_, err = swfClient.Client.RespondActivityTaskFailed(&swf.RespondActivityTaskFailedInput{
		TaskToken: &token,
		Reason:    aws.String(message),
		Details:   aws.String(string(failureJson)),
	})
	return
}
