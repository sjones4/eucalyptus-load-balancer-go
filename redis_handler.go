// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"time"
)

const (
	// Redis PUBLISH command for sending values
	PUBLISH = "PUBLISH"

	// Redis BLPOP command for receiving values
	BLPOP = "BLPOP"
)

// ActivityHandler implementation using Redis
type RedisHandler struct {
	conn redis.Conn
}

// Create a new Redis ActivityHandler.
func NewRedisHandler() (ActivityHandler, error) {
	conn, err := redis.Dial("tcp", ":6379",
		redis.DialConnectTimeout(seconds(60)),
		redis.DialReadTimeout(seconds(30)))
	if err != nil {
		return nil, err
	}
	return &RedisHandler{conn}, nil
}

func (handler *RedisHandler) Send(name string, value string) error {
	_, err := handler.conn.Do(PUBLISH, name, value)
	return err
}

func (handler *RedisHandler) Receive(name string) (*string, error) {
	popped, err := handler.conn.Do(BLPOP, fmt.Sprintf("%s-reply", name), 0)
	if err != nil {
		return nil, err
	}
	poppedArray, ok := popped.([]interface{})
	var resultString string
	if ok && len(poppedArray) > 1 {
		resultString = fmt.Sprintf("%s", poppedArray[1])
	} else {
		resultString = fmt.Sprintf("%s", popped)
	}
	return &resultString, nil
}

func (handler *RedisHandler) Close() {
	_ = handler.conn.Close()
}

func seconds(durationSeconds int64) time.Duration {
	return time.Duration(int64(time.Second) * durationSeconds)
}
