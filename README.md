# tinyurl

A URL shortener written in Go.

It supports five capabilities: shorten, redirect, expiry, custom alias and delete.
Codes are 7 Base62 characters. Links are stored in DynamoDB and cached in Redis.
Build with `go build ./...`.
