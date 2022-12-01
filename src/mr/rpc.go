package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import "os"
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//


// Add your RPC definitions here.
type RPCArgs struct {
	Id int	// 编号
	Phase State	// 状态
	OutPaths []Response // 输出路径
}

/*
	// 我的mapworker在完成任务之后，需要返回给master信息
	// 这些信息包括 
	// id 任务的名称
	// state 任务的状态
	// outputfiles 任务最后的输出文件在哪里
*/
type RPCReply struct {
	Phase State	// 状态
	HoldTask bool // 是否持有任务
	Nreduce int 
	MapTask MapTask
	ReduceTask MapTask
}

type Response struct{
	Id int // 编号
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
