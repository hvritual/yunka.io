#!/bin/bash

source ~/.bash_profile

function initPb()
{
    rm -rf client meta method server handle transport/memory transport/grpc transport
    mkdir -p client meta method server handle transport transport/memory transport/grpc
    cd pb
    protoc -I . -I ../../../ --go_out=plugins=grpc:../meta ./*.proto
    protoc -I "." -I ../../../ --xr-cluster_out=type=client,package=client,framework=yunka.io,rpc=yunka.io/gateway,project=yunka.io/gateway:../client ./*.proto
    protoc -I "." -I ../../../ --xr-cluster_out=type=method,package=method,framework=yunka.io,rpc=yunka.io/gateway,project=yunka.io/gateway:../method ./*.proto
    protoc -I "." -I ../../../ --xr-cluster_out=type=handle,package=handle,framework=yunka.io,rpc=yunka.io/gateway,project=yunka.io/gateway:../handle ./*.proto
    protoc -I "." -I ../../../ --xr-cluster_out=type=server,package=server,framework=yunka.io,rpc=yunka.io/gateway,project=yunka.io/gateway:../server ./*.proto
    protoc -I "." -I ../../../ --xr-cluster_out=type=memory,package=memory,framework=yunka.io,rpc=yunka.io/gateway,project=yunka.io/gateway:../transport/memory ./*.proto
    protoc -I "." -I ../../../ --xr-cluster_out=type=grpc,package=grpc,framework=yunka.io,rpc=yunka.io/gateway,project=yunka.io/gateway:../transport/grpc ./*.proto
    cd -

}

initPb

