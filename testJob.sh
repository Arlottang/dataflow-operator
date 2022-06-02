#!/bin/sh
# shellcheck disable=SC2068
kubectl run --rm -it --image=hub.pingcap.net/ticdc/dataflow:20220531-2 --restart=Never master-client -n dev -- /df-master-client ./testJob.sh --master-addr server-master:10240 submit-job --job-type FakeJob --job-config config/test/jobDemo.json