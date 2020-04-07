// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"errors"
	"fmt"
)

// ActivityHandler implementation using Channels
type ChannelHandler struct {
	Channels map[string]chan string
}

// ActivityHandler implementation using underlying handlers
type CompositeHandler struct {
	Handlers []ActivityHandler
}

// Create a new Channel backed ActivityHandler.
func NewChannelHandler(channels map[string]chan string) ActivityHandler {
	return &ChannelHandler{channels}
}

func (handler *ChannelHandler) Send(name string, value string) error {
	if channel, ok := handler.Channels[name]; ok {
		channel <- value
		return nil
	} else {
		return errors.New(fmt.Sprintf("channel not found: %s", name))
	}
}

func (handler *ChannelHandler) Receive(name string) (*string, error) {
	if channel, ok := handler.Channels[name]; ok {
		resultString := <-channel
		return &resultString, nil
	} else {
		return nil, errors.New(fmt.Sprintf("channel not found: %s", name))
	}
}

func (handler *ChannelHandler) Close() {
}

// Create a CompositeHandler backed by the given handlers
// The primary handler is used for both send and receive. Secondary handlers
// are used for sending only (listeners)
// A failure of the primary send will prevent secondary sends
func NewCompositeHandler(primary ActivityHandler, secondaries ...ActivityHandler) ActivityHandler {
	Handler := &CompositeHandler{}
	Handler.Handlers = append(Handler.Handlers, primary)
	Handler.Handlers = append(Handler.Handlers, secondaries...)
	return Handler
}

func (handler *CompositeHandler) Send(name string, value string) error {
	err := handler.Handlers[0].Send(name, value)
	if err == nil {
		for _, secondary := range handler.Handlers[1:] {
			_ = secondary.Send(name, value)
		}
	}

	return err
}

func (handler *CompositeHandler) Receive(name string) (*string, error) {
	result, err := handler.Handlers[0].Receive(name)
	return result, err
}

func (handler *CompositeHandler) Close() {
}
