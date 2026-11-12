package main

//go:generate protoc -I ../../api --go_out=../../pkg/calendarpb --go_opt=paths=source_relative --go-grpc_out=../../pkg/calendarpb --go-grpc_opt=paths=source_relative EventService.proto
