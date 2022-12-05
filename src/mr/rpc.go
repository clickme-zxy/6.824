package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

/*
 1. the master server
 2. worker server ask the master for the task
 3. the master first give the worker maptstask
 4. the worker finish the map task with the master listen 10s
 5. the worker ask the master for the new task and give the master the infomaton
*/

/*
	the worker call the function,given the RPCArgs and ask for the RPCReply from the master
	the worker will ask the master for the tak
*/
type RPCArgs struct {
	Id       int        // 编号
	Phase    State      // 状态
	OutPaths []Response // 输出路径
}

/*
	the master will response to the worker call with the info
*/
type RPCReply struct {
	Phase      State      // the phase of the overall job
	HoldTask   bool       // if there any job need to given
	Nreduce    int        // the number of the nreduce jobs
	MapTask    MapTask    // the maptask
	ReduceTask ReduceTask // the ReduceTask
}

/*
	the response is for the reduce task
		the id is the id of the reduce task
		the outpath is the id of the filename
	when a map task is over:
		it will produce many response for the reduce task
	and in the shuttle we will reoriginize the infor of the reduce task
*/

type Response struct {
	Id      int    // 编号
	OutPath string //输出的路径
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
