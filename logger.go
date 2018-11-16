package main

import (
	"fmt"
	"log"

	ilog "github.com/Loofort/ios-back/log"
)

type myLogger struct {
	with string
}

func (l myLogger) Info(message string, keyValues ...interface{}) {
	kv := splitKV(keyValues)
	fmt.Printf("%s: %s%s\n", message, l.with, kv)
}
func (l myLogger) Error(message string, keyValues ...interface{}) {
	kv := splitKV(keyValues)
	log.Printf("%s: %s%s\n", message, l.with, kv)
}
func (l myLogger) With(keyValues ...interface{}) ilog.Logger {
	l.with += splitKV(keyValues)
	return l
}

func splitKV(keyValues []interface{}) string {
	if len(keyValues)%2 != 0 {
		keyValues = append(keyValues, "nil")
	}
	str := ""
	for i := 0; i < len(keyValues); i += 2 {
		str += fmt.Sprintf("%s=%v, ", keyValues[i], keyValues[i+1])
	}
	return str
}
