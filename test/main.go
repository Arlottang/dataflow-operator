package main

import (
	"fmt"
	"strconv"
	"strings"
)

func main() {
	var num int32
	num = 3

	res := loadCommand3(&num)

	fmt.Println(res)

}

func loadCommand(num *int32) []string {
	listenStr := "server-master-" + strconv.Itoa(int(*(num))-1) + "." + "server-master"

	origin := `until nslookup tmp; 
do 
	echo waiting for tmp; 
	sleep 2; 
done;`

	currentStr := strings.Replace(origin, "tmp", listenStr, -1)

	return []string{
		"sh", "-c",
		currentStr,
	}
}

func loadCommand2(num *int32) []string {

	joinStr := ""

	numT := int(*(num))

	for i := 0; i < numT; i++ {
		joinStr += "server-master-" + strconv.Itoa(i) + "." + "server-master" + ":10240"
		if i == numT-1 {
			break
		}
		joinStr += ","
	}
	return []string{
		"/df-executor",
		"--config", "/mnt/config-map/config.toml",
		"--join", joinStr,
		"--worker-addr", "0.0.0.0:10241",
		"--advertise-addr", "${POD_HOSTNAME}.${EXECUTOR-SERVICE}:10241",
	}
}

func loadCommand3(size *int32) []string {
	initStr := ""

	num := int(*(size))

	for i := 0; i < num; i++ {
		initStr += "server-master-" + strconv.Itoa(i) + "=" + "https://" + "server-master-" + strconv.Itoa(i)
		initStr += "." + "server-master" + ":8291"
		if i == num-1 {
			break
		}
		initStr += ","
	}

	return []string{
		"/df-master",
		"--name", "${POD_HOSTNAME}",
		"--config", "/mnt/config-map/master.toml",
		"--master-addr", "0.0.0.0:10240",
		"--advertise-addr", "${POD_HOSTNAME}.server-master:10240",
		"--peer-urls", "http://127.0.0.1:8291",
		"--advertise-peer-urls", "http://${POD_HOSTNAME}.server-master:8291",
		"--initial-cluster", initStr,
		"--frame-meta-endpoints", "frame-mysql-standalone:3306",
		"--user-meta-endpoints", "user-etcd-standalone:2379",
	}
}
