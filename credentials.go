// Copyright (c) 2020 Steve Jones
// SPDX-License-Identifier: BSD-2-Clause

package main

import (
	"encoding/json"
	"os"
	"strings"
)

type Credentials struct {
	// Instances base-64 encoded PEM X.509 certificate
	InstancePublicKey string `json:"instance_pub_key"`

	// Instances base-64 encoded RSA private key
	InstancePrivateKey string `json:"instance_pk"`

	// Base64 encoded PEM X.509 certificate
	IamPublicKey string `json:"iam_pub_key"`

	IamToken string `json:"iam_token"`

	// Clouds base-64 encoded PEM X.509 certificate
	EucalyptusPublicKey string `json:"euca_pub_key"`
}

func CredentialString(credentialsText string) (credentials Credentials, err error) {
	credentials = Credentials{}
	err = json.Unmarshal([]byte(credentialsText), &credentials)
	if err == nil {
		credentials.Clean()
	}
	return
}

func CredentialFile(credentialsPath string) (credentials Credentials, err error) {
	credentialsFile, err := os.Open(credentialsPath)
	if err != nil {
		return
	}
	defer credentialsFile.Close()
	credentialsData := make([]byte, 1024*32)
	dataLength, err := credentialsFile.Read(credentialsData)
	if err != nil {
		return
	}
	credentials, err = CredentialString(string(credentialsData[:dataLength]))
	return
}

func (credentials *Credentials) Clean() {
	if credentials != nil {
		credentials.InstancePublicKey = strings.TrimSpace(credentials.InstancePublicKey)
		credentials.InstancePrivateKey = strings.TrimSpace(credentials.InstancePrivateKey)
		credentials.IamPublicKey = strings.TrimSpace(credentials.IamPublicKey)
		credentials.IamToken = strings.TrimSpace(credentials.IamToken)
		credentials.EucalyptusPublicKey = strings.TrimSpace(credentials.EucalyptusPublicKey)
	}
}
